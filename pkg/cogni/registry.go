package cogni

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Registry is the in-memory hot-pluggable catalog of Cogni declarations.
//
// It is the runtime counterpart of pkg/capsule.Registry: a Cogni declaration
// can be added/removed/enabled/disabled at any time without restarting the
// agent, and the Evaluator queries Registry.Active() each turn to decide
// which Cognis engage.
//
// Status (2026-04): this Registry replaces the historical pattern of
// passing a static []*Declaration slice into the Evaluator. It is safe for
// concurrent use.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*entry
	version int
	hooks   []ChangeHook
}

type entry struct {
	decl    *Declaration
	enabled bool
	source  string    // "file:/path/to/x.json", "inline", "api", ...
	loaded  time.Time
	loadErr string
}

// ChangeHook is called after every successful mutation. It runs while the
// registry's lock is held — keep handlers short and non-blocking.
type ChangeHook func(event, id string)

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]*entry)}
}

// Version returns a monotonically increasing counter that bumps on every
// mutation. Consumers (e.g. planner prompt builders) can compare versions
// to invalidate cached projections.
func (r *Registry) Version() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

// OnChange subscribes to mutations.
func (r *Registry) OnChange(fn ChangeHook) {
	r.mu.Lock()
	r.hooks = append(r.hooks, fn)
	r.mu.Unlock()
}

// Add registers a declaration under its ID. If a declaration with the same
// ID already exists, it is replaced (Update semantics).
func (r *Registry) Add(d *Declaration, source string) error {
	if d == nil {
		return fmt.Errorf("cogni.registry: nil declaration")
	}
	if err := d.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	event := "add"
	if _, ok := r.entries[d.ID]; ok {
		event = "update"
	}
	r.entries[d.ID] = &entry{
		decl:    d,
		enabled: true,
		source:  source,
		loaded:  time.Now(),
	}
	r.version++
	r.notifyLocked(event, d.ID)
	return nil
}

// Remove deletes a declaration. Returns false if no such ID was registered.
func (r *Registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[id]; !ok {
		return false
	}
	delete(r.entries, id)
	r.version++
	r.notifyLocked("remove", id)
	return true
}

// SetEnabled toggles a declaration's enabled flag. Disabled declarations
// remain registered but are skipped by Active() / All().
func (r *Registry) SetEnabled(id string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("cogni.registry: %q not found", id)
	}
	if e.enabled == enabled {
		return nil
	}
	e.enabled = enabled
	r.version++
	if enabled {
		r.notifyLocked("enable", id)
	} else {
		r.notifyLocked("disable", id)
	}
	return nil
}

// IsEnabled reports whether a declaration is currently enabled.
// Returns false if the ID is not registered.
func (r *Registry) IsEnabled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	return ok && e.enabled
}

// Get returns a declaration by ID. The bool reports whether it was found
// (regardless of enabled state).
func (r *Registry) Get(id string) (*Declaration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	if !ok {
		return nil, false
	}
	return e.decl, true
}

// Active returns all enabled declarations, sorted by ID for stable output.
// This is the slice typically passed to Evaluator.Evaluate().
func (r *Registry) Active() []*Declaration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Declaration, 0, len(r.entries))
	for _, e := range r.entries {
		if e.enabled {
			out = append(out, e.decl)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// EntryStatus is a serializable snapshot of one entry — used by the admin API.
type EntryStatus struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Capsule     string `json:"capsule,omitempty"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source,omitempty"`
	LoadedAt    string `json:"loaded_at,omitempty"`
	LoadError   string `json:"load_error,omitempty"`
	Priority    int    `json:"priority"`
	Exclusive   string `json:"exclusive,omitempty"`
	AlwaysOn    bool   `json:"always_on"`
}

// List returns a snapshot of every registered entry (enabled or not),
// sorted by ID.
func (r *Registry) List() []EntryStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EntryStatus, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, statusOf(e))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ReloadFromDir replaces the file-sourced subset of entries with what is
// currently on disk. Inline / API-added entries are preserved. The function
// reports the resulting summary so the caller can surface load errors.
//
// Reload semantics:
//
//	for each .json file in dir:
//	    parse → Add (update or insert) with source="file:<abs>"
//	for each existing entry whose source begins with "file:" and whose path
//	    is no longer present on disk:
//	    Remove
func (r *Registry) ReloadFromDir(dir string) (ReloadSummary, error) {
	decls, loadErrs, err := LoadDeclarationsFromDir(dir)
	if err != nil {
		return ReloadSummary{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	eval := NewEvaluator()
	seen := make(map[string]bool, len(decls))
	added, updated := 0, 0
	var checkFailures []CheckResult

	stamp := func(d *Declaration, e *entry) {
		// Run self-tests (if any) and record failures onto the entry so
		// the admin UI can surface "this cogni has broken checks" without
		// another round trip. Failing checks do NOT block the reload —
		// the declaration still loads; the operator is expected to fix
		// the assertion or the declaration.
		results := VerifyDeclaration(d, eval)
		var fails []CheckResult
		for _, rr := range results {
			if !rr.Passed && rr.Reason != "no assertion configured (ignored)" {
				fails = append(fails, rr)
			}
		}
		if len(fails) > 0 {
			checkFailures = append(checkFailures, fails...)
			summary := make([]string, 0, len(fails))
			for _, f := range fails {
				label := f.CheckName
				if label == "" {
					label = fmt.Sprintf("check[%d]", f.CheckIndex)
				}
				summary = append(summary, label+": "+f.Reason)
			}
			e.loadErr = "checks failed: " + strings.Join(summary, "; ")
		} else {
			e.loadErr = ""
		}
	}

	for _, d := range decls {
		seen[d.ID] = true
		if existing, ok := r.entries[d.ID]; ok {
			existing.decl = d
			existing.loaded = time.Now()
			stamp(d, existing)
			updated++
			r.notifyLocked("update", d.ID)
		} else {
			ent := &entry{
				decl:    d,
				enabled: true,
				source:  "file:" + dir,
				loaded:  time.Now(),
			}
			stamp(d, ent)
			r.entries[d.ID] = ent
			added++
			r.notifyLocked("add", d.ID)
		}
	}

	removed := 0
	for id, e := range r.entries {
		if strings.HasPrefix(e.source, "file:") && !seen[id] {
			delete(r.entries, id)
			removed++
			r.notifyLocked("remove", id)
		}
	}

	if added+updated+removed > 0 {
		r.version++
	}
	return ReloadSummary{
		Added:         added,
		Updated:       updated,
		Removed:       removed,
		Errors:        loadErrs,
		CheckFailures: checkFailures,
	}, nil
}

// ReloadSummary describes the effect of a ReloadFromDir call.
type ReloadSummary struct {
	Added         int           `json:"added"`
	Updated       int           `json:"updated"`
	Removed       int           `json:"removed"`
	Errors        []LoadError   `json:"errors,omitempty"`
	CheckFailures []CheckResult `json:"check_failures,omitempty"`
}

func (r *Registry) notifyLocked(event, id string) {
	hooks := append([]ChangeHook(nil), r.hooks...)
	for _, h := range hooks {
		h(event, id)
	}
}

func statusOf(e *entry) EntryStatus {
	s := EntryStatus{
		ID:        e.decl.ID,
		Enabled:   e.enabled,
		Source:    e.source,
		LoadError: e.loadErr,
		Priority:  e.decl.Priority,
		Exclusive: e.decl.Exclusive,
	}
	if e.decl.DisplayName != "" {
		s.DisplayName = e.decl.DisplayName
	}
	if e.decl.Description != "" {
		s.Description = e.decl.Description
	}
	if e.decl.Capsule != "" {
		s.Capsule = e.decl.Capsule
	}
	s.AlwaysOn = e.decl.Activation.AlwaysOn
	if !e.loaded.IsZero() {
		s.LoadedAt = e.loaded.UTC().Format(time.RFC3339)
	}
	return s
}
