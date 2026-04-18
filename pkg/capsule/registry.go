package capsule

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/skills"
)

// Registry is the central catalog of Capsules. It owns lifecycle transitions,
// slot bookkeeping, dependency resolution, and aggregation of exports.
//
// Registry is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	entries  map[CapsuleID]*entry
	slots    map[string]CapsuleID // slot → owner capsule ID
	version  int                  // bumps on any mutation (for cache invalidation)
	observers []RegistryObserver
}

// RegistryObserver is notified on every registry mutation.
type RegistryObserver func(event string, id CapsuleID)

type entry struct {
	capsule   Capsule
	lifecycle *Lifecycle
	enabled   bool
}

// NewRegistry creates an empty capsule registry.
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[CapsuleID]*entry),
		slots:   make(map[string]CapsuleID),
	}
}

// Version is a monotonically increasing counter that changes on any mutation.
// Consumers (planner prompt cache, UI) can compare versions to decide whether
// to refresh cached projections.
func (r *Registry) Version() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

// OnChange subscribes to registry mutations.
func (r *Registry) OnChange(fn RegistryObserver) {
	r.mu.Lock()
	r.observers = append(r.observers, fn)
	r.mu.Unlock()
}

// Register adds a Capsule to the registry in the Installed state.
// Built-in capsules skip the Installing phase because their artifacts
// are bundled into the host binary.
//
// Register returns an error if the Capsule is invalid or if its exclusive
// slot is already occupied by a different capsule.
func (r *Registry) Register(c Capsule) error {
	if err := Validate(c); err != nil {
		return err
	}
	m := c.Manifest()
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[m.Name]; exists {
		return fmt.Errorf("capsule %q already registered", m.Name)
	}

	if m.Slot != "" {
		if owner, taken := r.slots[m.Slot]; taken {
			return fmt.Errorf("slot %q already occupied by %q", m.Slot, owner)
		}
		r.slots[m.Slot] = m.Name
	}

	lc := NewLifecycle()
	_ = lc.Transition(StateInstalling, "register")
	_ = lc.Transition(StateInstalled, "builtin artifacts")

	r.entries[m.Name] = &entry{
		capsule:   c,
		lifecycle: lc,
		enabled:   true, // built-ins default to enabled
	}
	r.version++
	r.notifyLocked("register", m.Name)
	return nil
}

// Unregister removes a capsule from the registry. If it is still activated,
// it is first suspended. The exclusive slot is freed.
func (r *Registry) Unregister(ctx context.Context, id CapsuleID) error {
	r.mu.Lock()
	e, ok := r.entries[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("capsule %q not found", id)
	}
	r.mu.Unlock()

	if e.lifecycle.State() == StateActivated {
		if err := r.Suspend(ctx, id, "unregister"); err != nil {
			return err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	m := e.capsule.Manifest()
	if m.Slot != "" {
		delete(r.slots, m.Slot)
	}
	_ = e.lifecycle.Transition(StateUninstalling, "unregister")
	_ = e.lifecycle.Transition(StateRegistered, "unregister")
	delete(r.entries, id)
	r.version++
	r.notifyLocked("unregister", id)
	return nil
}

// Enable marks a capsule as enabled (subject to future activation).
func (r *Registry) Enable(id CapsuleID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("capsule %q not found", id)
	}
	e.enabled = true
	if e.lifecycle.State() == StateInstalled {
		_ = e.lifecycle.Transition(StateEnabled, "enable")
	}
	r.version++
	r.notifyLocked("enable", id)
	return nil
}

// Disable marks a capsule as disabled. If it is currently activated, it is
// also suspended.
func (r *Registry) Disable(ctx context.Context, id CapsuleID) error {
	r.mu.Lock()
	e, ok := r.entries[id]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("capsule %q not found", id)
	}

	if e.lifecycle.State() == StateActivated {
		if err := r.Suspend(ctx, id, "disable"); err != nil {
			return err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	e.enabled = false
	if e.lifecycle.State() == StateEnabled {
		_ = e.lifecycle.Transition(StateInstalled, "disable")
	}
	r.version++
	r.notifyLocked("disable", id)
	return nil
}

// Activate starts the capsule's runtime. If the capsule is currently
// Installed, it is first enabled. If it is Suspended or Failed, a transition
// is attempted back to Enabled before activation.
func (r *Registry) Activate(ctx context.Context, id CapsuleID, env *Env) error {
	r.mu.Lock()
	e, ok := r.entries[id]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("capsule %q not found", id)
	}

	state := e.lifecycle.State()
	switch state {
	case StateActivated:
		return nil // idempotent
	case StateInstalled:
		if err := e.lifecycle.Transition(StateEnabled, "auto-enable for activate"); err != nil {
			return err
		}
	case StateSuspended, StateFailed:
		if err := e.lifecycle.Transition(StateEnabled, "resume for activate"); err != nil {
			return err
		}
	case StateEnabled:
		// Already enabled, proceed.
	default:
		return fmt.Errorf("cannot activate capsule in state %s", state)
	}

	if err := e.capsule.Runtime().Start(ctx, env); err != nil {
		_ = e.lifecycle.Fail(err)
		return fmt.Errorf("capsule %q runtime start: %w", id, err)
	}
	if err := e.lifecycle.Transition(StateActivated, "runtime started"); err != nil {
		return err
	}

	r.mu.Lock()
	r.version++
	r.notifyLocked("activate", id)
	r.mu.Unlock()
	return nil
}

// Suspend stops the capsule's runtime, leaving it in the Suspended state
// (resumable via Activate).
func (r *Registry) Suspend(ctx context.Context, id CapsuleID, reason string) error {
	r.mu.Lock()
	e, ok := r.entries[id]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("capsule %q not found", id)
	}

	if e.lifecycle.State() != StateActivated {
		return nil // idempotent for non-activated
	}

	if err := e.capsule.Runtime().Stop(ctx); err != nil {
		_ = e.lifecycle.Fail(err)
		return fmt.Errorf("capsule %q runtime stop: %w", id, err)
	}
	if err := e.lifecycle.Transition(StateSuspended, reason); err != nil {
		return err
	}

	r.mu.Lock()
	r.version++
	r.notifyLocked("suspend", id)
	r.mu.Unlock()
	return nil
}

// Get returns a capsule by ID (enabled or not).
func (r *Registry) Get(id CapsuleID) (Capsule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	if !ok {
		return nil, false
	}
	return e.capsule, true
}

// State returns the lifecycle state of a capsule.
func (r *Registry) State(id CapsuleID) (State, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	if !ok {
		return "", false
	}
	return e.lifecycle.State(), true
}

// All returns all capsules whose state is Enabled or Activated.
// This is the set of capsules that contribute exports to the host.
func (r *Registry) All() []Capsule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Capsule, 0, len(r.entries))
	for _, e := range r.entries {
		if !e.enabled {
			continue
		}
		s := e.lifecycle.State()
		if s == StateEnabled || s == StateActivated {
			out = append(out, e.capsule)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Manifest().Name < out[j].Manifest().Name
	})
	return out
}

// Descriptors returns a descriptor for every registered capsule (including
// disabled ones), sorted by ID. Used by the admin UI.
func (r *Registry) Descriptors() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.entries))
	for _, e := range r.entries {
		source := e.capsule.Manifest().Source
		if source == "" {
			source = "builtin"
		}
		out = append(out, DescribeCapsule(e.capsule, e.lifecycle.State(), e.enabled, source))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// AllSkills returns the union of skills from every currently-active capsule.
// Skills are deduplicated by name; first occurrence wins.
func (r *Registry) AllSkills() []skills.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	var out []skills.Skill
	for _, e := range r.entries {
		if !e.enabled {
			continue
		}
		s := e.lifecycle.State()
		if s != StateEnabled && s != StateActivated {
			continue
		}
		exp := e.capsule.Exports()
		if exp == nil {
			continue
		}
		for _, sk := range exp.Skills {
			if seen[sk.Name()] {
				continue
			}
			seen[sk.Name()] = true
			out = append(out, sk)
		}
	}
	return out
}

// AllSkillsByCapsule returns skill candidates annotated with their owning
// capsule ID, suitable for feeding into cogni.Surface().
func (r *Registry) AllSkillsByCapsule() []cogni.SurfaceInput {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []cogni.SurfaceInput
	for _, e := range r.entries {
		if !e.enabled {
			continue
		}
		s := e.lifecycle.State()
		if s != StateEnabled && s != StateActivated {
			continue
		}
		exp := e.capsule.Exports()
		if exp == nil {
			continue
		}
		id := e.capsule.Manifest().Name
		for _, sk := range exp.Skills {
			out = append(out, cogni.SurfaceInput{Skill: sk, Capsule: id})
		}
	}
	return out
}

// CombinedSystemPrompt concatenates the SystemPrompt from every active capsule.
// Empty prompts are skipped; each section is preceded by "## {name} 领域能力".
func (r *Registry) CombinedSystemPrompt() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out string
	for _, e := range r.entries {
		if !e.enabled {
			continue
		}
		s := e.lifecycle.State()
		if s != StateEnabled && s != StateActivated {
			continue
		}
		exp := e.capsule.Exports()
		if exp == nil || exp.SystemPrompt == "" {
			continue
		}
		out += "\n\n## " + e.capsule.Manifest().DisplayOrName() + " 领域能力\n" + exp.SystemPrompt
	}
	return out
}

// AllUITabs returns every UI tab contributed by active capsules, with the
// Capsule field back-filled to the owning capsule ID.
func (r *Registry) AllUITabs() []UITab {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var tabs []UITab
	for _, e := range r.entries {
		if !e.enabled {
			continue
		}
		s := e.lifecycle.State()
		if s != StateEnabled && s != StateActivated {
			continue
		}
		exp := e.capsule.Exports()
		if exp == nil {
			continue
		}
		for _, t := range exp.UITabs {
			t.Capsule = e.capsule.Manifest().Name
			tabs = append(tabs, t)
		}
	}
	return tabs
}

// AllHTTPRoutes returns "/v1/ext/{capsule}{path}" → handler for every active
// capsule's exports.HTTPRoutes.
func (r *Registry) AllHTTPRoutes() map[string]http.HandlerFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]http.HandlerFunc)
	for _, e := range r.entries {
		if !e.enabled {
			continue
		}
		s := e.lifecycle.State()
		if s != StateEnabled && s != StateActivated {
			continue
		}
		exp := e.capsule.Exports()
		if exp == nil {
			continue
		}
		for p, h := range exp.HTTPRoutes {
			out["/v1/ext/"+e.capsule.Manifest().Name+p] = h
		}
	}
	return out
}

// AllCogniDeclarations collects every Cogni declaration exposed by active
// capsules (either inline via Exports().Cogni or file-referenced).
func (r *Registry) AllCogniDeclarations() []*cogni.Declaration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*cogni.Declaration
	for _, e := range r.entries {
		if !e.enabled {
			continue
		}
		s := e.lifecycle.State()
		if s != StateEnabled && s != StateActivated {
			continue
		}
		exp := e.capsule.Exports()
		if exp == nil || exp.Cogni == nil {
			continue
		}
		decl := exp.Cogni
		if decl.Capsule == "" {
			decl.Capsule = e.capsule.Manifest().Name
		}
		out = append(out, decl)
	}
	return out
}

// SlotOwner returns the capsule occupying a given slot.
func (r *Registry) SlotOwner(slot string) (CapsuleID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.slots[slot]
	return id, ok
}

func (r *Registry) notifyLocked(event string, id CapsuleID) {
	observers := append([]RegistryObserver(nil), r.observers...)
	for _, fn := range observers {
		fn(event, id)
	}
}
