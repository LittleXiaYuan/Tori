package cogni

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"text/template"
	"time"

	"yunque-agent/pkg/skills"
)

// MemorySearchFunc plugs the host's memory-recall primitive into the Cogni
// runtime so Declaration.Context.MemoryQuery declarations actually hit the
// same retrieval pipeline the planner uses.
//
// The signature intentionally mirrors the planner's existing memory hook
// (see cmd/agent/init_planner.go) so wiring is a one-liner: nil disables
// MemoryQuery rendering entirely.
type MemorySearchFunc func(ctx context.Context, tenantID, query string) string

// SkillOwnerFunc resolves the "owning capsule" of a skill, used to make
// Declaration.Surface.FromCapsules actually filter. Until a real
// pkg/capsule.Registry is wired into the runtime, the practical source
// of truth is the skill category (see pkg/skills.Registry.CategoryOf).
// A nil callback leaves every SurfaceInput's Capsule field empty,
// matching the previous behaviour where FromCapsules was inert.
type SkillOwnerFunc func(skillName string) string

// ExperienceProvider resolves the ExperienceStore for a given cogni ID.
// Returns nil when no experience is configured for that cogni.
type ExperienceProvider func(cogniID string) *ExperienceStore

// Hook bridges a Registry to the planner: each turn the planner asks the
// hook which cognis activate for the current Session, what context blob to
// inject, and (Phase 1b) which tool surface to expose.
//
// Hook owns an Evaluator and caches compiled templates internally.
// It is safe for concurrent use.
//
// When a TraceStore is attached, every turn emits one Trace describing
// activation scoring, exclusivity decisions, context-block assembly, and
// the tool-filter diff. Tracing is opt-in; nil store == zero overhead.
type Hook struct {
	registry *Registry
	eval     *Evaluator

	tmplMu sync.Mutex
	tmpls  map[string]*template.Template

	traceMu sync.RWMutex
	traces  TraceStore

	memMu        sync.RWMutex
	memorySearch MemorySearchFunc

	ownerMu    sync.RWMutex
	skillOwner SkillOwnerFunc

	expMu       sync.RWMutex
	expProvider ExperienceProvider

	actMu        sync.RWMutex
	onActivation func(cogniID string, score float64)

	embedMu  sync.RWMutex
	embedder EmbedderFunc

	arbMu       sync.RWMutex
	arbitration ArbitrationConfig

	expTuneMu        sync.RWMutex
	experienceTuning ExperienceTuningConfig

	turnCache *turnCache
}

// SetExperienceTuning enables experience-driven surface pruning (drop a cogni's
// consistently-failing tools from its surface). The zero config (default) keeps
// legacy behavior. Intended to be set once at wiring time.
func (h *Hook) SetExperienceTuning(cfg ExperienceTuningConfig) {
	if h == nil {
		return
	}
	h.expTuneMu.Lock()
	h.experienceTuning = cfg
	h.expTuneMu.Unlock()
}

func (h *Hook) experienceTuningCfg() ExperienceTuningConfig {
	if h == nil {
		return ExperienceTuningConfig{}
	}
	h.expTuneMu.RLock()
	defer h.expTuneMu.RUnlock()
	return h.experienceTuning
}

// RecordToolOutcome attributes a tool execution result to every activated cogni
// whose non-identity surface includes that tool and that has an experience
// store, feeding the self-tuning loop. Cheap: activation is turn-cached and the
// store debounces its disk writes. No-op when experience isn't wired.
func (h *Hook) RecordToolOutcome(req ContextRequest, tool string, success bool) {
	if h == nil {
		return
	}
	provider := h.experienceProviderFn()
	if provider == nil {
		return
	}
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return
	}
	for _, a := range h.Activate(req) {
		d := a.Declaration
		if d == nil || isIdentitySurface(d.Surface) || !d.Surface.AllowsName(tool) {
			continue
		}
		if store := provider(d.ID); store != nil {
			store.RecordToolOutcome(tool, success)
		}
	}
}

// pruneByExperience drops tools from a single cogni's surfaced set when its own
// experience shows them consistently failing (>= MinObservations observations
// with success rate < MinSuccessRate). No-op when tuning is disabled, no
// experience store exists, or there is no data — and never prunes to empty (a
// degenerate surface would lock the model out of every tool).
func (h *Hook) pruneByExperience(cogniID string, in []skills.Skill) []skills.Skill {
	cfg := h.experienceTuningCfg()
	if cfg.IsZero() || len(in) == 0 {
		return in
	}
	provider := h.experienceProviderFn()
	if provider == nil {
		return in
	}
	store := provider(cogniID)
	if store == nil {
		return in
	}
	out := make([]skills.Skill, 0, len(in))
	for _, sk := range in {
		rate, count, ok := store.ToolSuccess(sk.Name())
		if ok && count >= cfg.MinObservations && rate < cfg.MinSuccessRate {
			slog.Debug("cogni: pruning low-success tool from surface",
				"cogni", cogniID, "tool", sk.Name(), "rate", rate, "obs", count)
			continue
		}
		out = append(out, sk)
	}
	if len(out) == 0 {
		return in
	}
	return out
}

// SetArbitration enables per-turn capability arbitration (top-K bidding +
// confidence floor). The zero config (default) keeps the legacy behavior where
// every activated cogni composes. Intended to be set once at wiring time.
func (h *Hook) SetArbitration(cfg ArbitrationConfig) {
	if h == nil {
		return
	}
	h.arbMu.Lock()
	h.arbitration = cfg
	h.arbMu.Unlock()
}

func (h *Hook) arbitrationCfg() ArbitrationConfig {
	if h == nil {
		return ArbitrationConfig{}
	}
	h.arbMu.RLock()
	defer h.arbMu.RUnlock()
	return h.arbitration
}

// SetEmbedder wires the host embedder used for semantic Cogni activation. It is
// propagated to the Evaluator (which lazily caches each Cogni's example vector).
// Passing nil disables semantic activation, leaving keyword/regex scoring intact.
func (h *Hook) SetEmbedder(fn EmbedderFunc) {
	if h == nil {
		return
	}
	h.embedMu.Lock()
	h.embedder = fn
	h.embedMu.Unlock()
	if h.eval != nil {
		h.eval.SetEmbedder(fn)
	}
}

func (h *Hook) embedderFn() EmbedderFunc {
	if h == nil {
		return nil
	}
	h.embedMu.RLock()
	defer h.embedMu.RUnlock()
	return h.embedder
}

// NewHook constructs a Hook around the given Registry.
func NewHook(r *Registry) *Hook {
	if r == nil {
		return nil
	}
	return &Hook{
		registry:  r,
		eval:      NewEvaluator(),
		tmpls:     make(map[string]*template.Template),
		turnCache: newTurnCache(turnCacheTTL),
	}
}

// turnCacheTTL caps how long an evaluation snapshot is reused across the
// two planner callbacks (BuildContext and FilterSkills). It only needs to
// outlive a single planner.Run, but a few seconds of slop tolerates retries
// and slow LLM calls without re-evaluating identical inputs.
const turnCacheTTL = 30 * time.Second

// SetMemorySearch attaches the host's memory recall primitive. Pass nil
// to disable Declaration.Context.MemoryQuery rendering.
func (h *Hook) SetMemorySearch(fn MemorySearchFunc) {
	if h == nil {
		return
	}
	h.memMu.Lock()
	h.memorySearch = fn
	h.memMu.Unlock()
}

func (h *Hook) memorySearchFn() MemorySearchFunc {
	if h == nil {
		return nil
	}
	h.memMu.RLock()
	defer h.memMu.RUnlock()
	return h.memorySearch
}

// SetExperienceProvider attaches the per-cogni experience lookup so
// ContextHints can be injected into the system prompt when a cogni activates.
func (h *Hook) SetExperienceProvider(fn ExperienceProvider) {
	if h == nil {
		return
	}
	h.expMu.Lock()
	h.expProvider = fn
	h.expMu.Unlock()
}

// SetOnActivation registers a callback invoked whenever a cogni activates.
func (h *Hook) SetOnActivation(fn func(cogniID string, score float64)) {
	if h == nil {
		return
	}
	h.actMu.Lock()
	h.onActivation = fn
	h.actMu.Unlock()
}

func (h *Hook) fireActivation(cogniID string, score float64) {
	if h == nil {
		return
	}
	h.actMu.RLock()
	fn := h.onActivation
	h.actMu.RUnlock()
	if fn != nil {
		fn(cogniID, score)
	}
}

func (h *Hook) experienceProviderFn() ExperienceProvider {
	if h == nil {
		return nil
	}
	h.expMu.RLock()
	defer h.expMu.RUnlock()
	return h.expProvider
}

// SetSkillOwner attaches the skill→capsule lookup used by ToolSurface.FromCapsules.
// Passing nil disables FromCapsules filtering (treated as identity).
func (h *Hook) SetSkillOwner(fn SkillOwnerFunc) {
	if h == nil {
		return
	}
	h.ownerMu.Lock()
	h.skillOwner = fn
	h.ownerMu.Unlock()
}

func (h *Hook) skillOwnerFn() SkillOwnerFunc {
	if h == nil {
		return nil
	}
	h.ownerMu.RLock()
	defer h.ownerMu.RUnlock()
	return h.skillOwner
}

// SetTraceStore attaches a sink for per-turn evaluation Traces. Pass nil
// to disable tracing.
func (h *Hook) SetTraceStore(s TraceStore) {
	if h == nil {
		return
	}
	h.traceMu.Lock()
	h.traces = s
	h.traceMu.Unlock()
}

// TraceStore returns the currently-attached store (may be nil).
func (h *Hook) TraceStore() TraceStore {
	if h == nil {
		return nil
	}
	h.traceMu.RLock()
	defer h.traceMu.RUnlock()
	return h.traces
}

// TraceSnapshot returns the current per-turn trace for a request fingerprint.
// It is intended for host runtimes that want to surface Cogni routing decisions
// inline in their execution trace. The method returns a copy and does not flush
// the trace store; FilterSkills/flushTrace remain responsible for persistence.
func (h *Hook) TraceSnapshot(req ContextRequest) (Trace, bool) {
	st := h.evaluate(req)
	if st == nil {
		return Trace{}, false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.trace, true
}

// ContextRequest captures the per-turn information needed to evaluate
// activation rules.
type ContextRequest struct {
	Message  string
	TenantID string
	Channel  string
	// Tags carries free-form session hints (e.g. "admin", "guest").
	Tags []string
	// PriorHandover is the set of handover tags emitted by Cognis that ran
	// earlier in the same turn (typically empty for single-turn requests).
	PriorHandover []string
}

// turnState is the shared evaluation snapshot for a single request fingerprint.
// BuildContext and FilterSkills both consult it so they never re-evaluate the
// same Session twice within one turn — and so the Trace we emit aggregates the
// entire decision (context bytes + tool filter diff) rather than splitting it
// across two records.
type turnState struct {
	mu           sync.Mutex
	created      time.Time
	activations  []Activation
	rawResults   []Activation // pre-exclusivity, for trace reasons
	trace        Trace
	traceEmitted bool
}

// Activate evaluates every active declaration and returns the post-exclusivity
// list of activated entries. Useful for audit, UI badges, and tool surface
// filtering. This is an alias for the Activations sub-result of evaluate().
func (h *Hook) Activate(req ContextRequest) []Activation {
	if h == nil || h.registry == nil {
		return nil
	}
	st := h.evaluate(req)
	if st == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := append([]Activation(nil), st.activations...)
	return out
}

// evaluate is the single entry point for Cogni rule evaluation.  It either
// returns a fresh turnState (cached for ~30s under a per-request key) or
// the cached one shared with the sibling callback within the same turn.
// FilterSkills flushes the Trace on completion; BuildContext only stamps
// the Context fields.
func (h *Hook) evaluate(req ContextRequest) *turnState {
	if h == nil || h.registry == nil {
		return nil
	}
	decls := h.registry.Active()
	if len(decls) == 0 {
		return nil
	}

	session := Session{
		Message:       req.Message,
		TenantID:      req.TenantID,
		Channel:       req.Channel,
		Tags:          req.Tags,
		PriorHandover: req.PriorHandover,
	}

	st := h.turnCache.getOrInit(req, func() *turnState {
		// Embed the message once per turn (only on cache-miss) so semantic
		// activation costs at most one embed call per turn, shared by
		// BuildContext/FilterSkills/Trace via the turn cache.
		if fn := h.embedderFn(); fn != nil && strings.TrimSpace(session.Message) != "" {
			session.MessageVec = fn(session.Message)
		}
		raw := h.eval.Evaluate(decls, session)
		// Track per-id raw activation + score so suppression can be reasoned
		// about even after exclusivity collapses the list.
		excl := ApplyExclusivity(raw)
		final := Filtered(excl)
		// Capability arbitration ("top-K experts win"): after exclusivity, cap
		// the composing set by bid (score) + confidence floor. Identity when no
		// host opted in, so legacy "all activated compose" is preserved.
		final = Arbitrate(final, h.arbitrationCfg())

		ts := &turnState{
			created:     time.Now(),
			activations: final,
			rawResults:  raw,
		}
		ts.trace = Trace{
			Timestamp:   ts.created,
			TenantID:    req.TenantID,
			Channel:     req.Channel,
			MessageHash: hashMessage(req.Message),
			MessageLen:  len([]rune(req.Message)),
			Activations: buildTraceActivations(raw, final),
		}
		return ts
	})

	return st
}

// flushTrace emits the trace once (idempotent) and computes total duration.
func (h *Hook) flushTrace(st *turnState) {
	if st == nil || h == nil {
		return
	}
	store := h.TraceStore()
	if store == nil {
		return
	}
	st.mu.Lock()
	if st.traceEmitted {
		st.mu.Unlock()
		return
	}
	st.traceEmitted = true
	st.trace.DurationMs = time.Since(st.created).Milliseconds()
	out := st.trace
	st.mu.Unlock()
	store.Record(out)
}

// buildTraceActivations records every evaluated cogni — including the ones
// suppressed by exclusivity — so operators can debug "why didn't X engage?".
func buildTraceActivations(raw, final []Activation) []TraceActivation {
	finalIDs := make(map[string]bool, len(final))
	for _, a := range final {
		if a.Declaration != nil {
			finalIDs[a.Declaration.ID] = true
		}
	}
	// group raw by Exclusive to identify the winning suppressor of each loser
	winnerByGroup := map[string]string{}
	for _, a := range final {
		if a.Declaration != nil && a.Declaration.Exclusive != "" {
			winnerByGroup[a.Declaration.Exclusive] = a.Declaration.ID
		}
	}

	out := make([]TraceActivation, 0, len(raw))
	for _, a := range raw {
		if a.Declaration == nil {
			continue
		}
		entry := TraceActivation{
			ID:          a.Declaration.ID,
			DisplayName: a.Declaration.DisplayName,
			Score:       round3(a.Score),
			Activated:   a.Activated && finalIDs[a.Declaration.ID],
			Reasons:     append([]string(nil), a.Reasons...),
		}
		if a.Activated && !finalIDs[a.Declaration.ID] && a.Declaration.Exclusive != "" {
			entry.Suppressed = true
			entry.SuppressedByID = winnerByGroup[a.Declaration.Exclusive]
		}
		out = append(out, entry)
	}
	return out
}

func round3(f float64) float64 {
	const k = 1000
	return float64(int(f*k+0.5)) / k
}

// BuildContext assembles the planner system-prompt addition from every
// activated cogni's ContextInjection. Returns "" when no cogni activates.
//
// Renders Static, Template, MemoryQuery (via host MemorySearch callback),
// and Experience hints for each activated declaration.
func (h *Hook) BuildContext(req ContextRequest) string {
	st := h.evaluate(req)
	if st == nil {
		return ""
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.activations) == 0 {
		return ""
	}
	var blocks []string
	var sources []string
	fallbacks := 0
	for _, a := range st.activations {
		h.fireActivation(a.Declaration.ID, a.Score)
		block, fellBack := h.renderContextOnce(a.Declaration, req)
		if block != "" {
			blocks = append(blocks, block)
			sources = append(sources, a.Declaration.ID)
		}
		if fellBack {
			fallbacks++
		}
	}
	if len(blocks) == 0 {
		return ""
	}
	out := "## 智体上下文\n" + strings.Join(blocks, "\n\n")
	st.trace.Context = TraceContext{
		Bytes:             len(out),
		Sources:           sources,
		TemplateFallbacks: fallbacks,
	}
	return out
}

// FilterSkills narrows the candidate skill list to the union of every
// activated cogni's ToolSurface. The contract is intentionally permissive
// to avoid breaking the agent in edge cases:
//
//   - hook is nil  → returns input unchanged
//   - no cogni activates  → returns input unchanged (no-op)
//   - all activated cognis have a zero-valued ToolSurface  → returns input
//     unchanged (every Surface acts as identity, so the union is the input)
//   - the union is empty (e.g. every cogni used `only:` with no overlap)
//     → returns input unchanged with a warn log (refuse to lock the model
//     out of every tool)
//
// When a SkillOwnerFunc is attached (via SetSkillOwner), each SurfaceInput's
// Capsule field is populated from the skill's owning category/capsule, making
// `FromCapsules` constraints in cogni declarations effective.
func (h *Hook) FilterSkills(req ContextRequest, in []skills.Skill) []skills.Skill {
	if h == nil || len(in) == 0 {
		return in
	}
	st := h.evaluate(req)
	if st == nil {
		return in
	}

	st.mu.Lock()
	acts := st.activations
	if len(acts) == 0 {
		st.mu.Unlock()
		h.flushTrace(st)
		return in
	}

	ownerFn := h.skillOwnerFn()
	candidates := make([]SurfaceInput, len(in))
	for i, s := range in {
		c := SurfaceInput{Skill: s}
		if ownerFn != nil {
			c.Capsule = ownerFn(s.Name())
		}
		candidates[i] = c
	}

	var allSurfaces [][]skills.Skill
	identityCount := 0
	var appliedBy []string
	for _, a := range acts {
		s := a.Declaration.Surface
		if isIdentitySurface(s) {
			identityCount++
			continue
		}
		copied := make([]SurfaceInput, len(candidates))
		copy(copied, candidates)
		surfaced := Surface(copied, s)
		// Self-tuning: drop this cogni's consistently-failing tools (no-op unless
		// experience tuning is enabled and the cogni has accumulated outcomes).
		surfaced = h.pruneByExperience(a.Declaration.ID, surfaced)
		allSurfaces = append(allSurfaces, surfaced)
		appliedBy = append(appliedBy, a.Declaration.ID)
	}

	tf := &TraceToolFilter{
		Before:          len(in),
		AppliedByCognis: appliedBy,
	}

	if len(allSurfaces) == 0 || identityCount == len(acts) {
		tf.After = len(in)
		st.trace.ToolFilter = tf
		st.mu.Unlock()
		h.flushTrace(st)
		return in
	}

	merged := MergeSurfaces(allSurfaces...)
	if len(merged) == 0 {
		slog.Warn("cogni: surface filter produced empty set; preserving original tool list",
			"activated", len(acts))
		tf.After = len(in)
		tf.FellBackToInput = true
		st.trace.ToolFilter = tf
		st.mu.Unlock()
		h.flushTrace(st)
		return in
	}
	tf.After = len(merged)
	tf.Removed = diffSkillNames(in, merged)
	st.trace.ToolFilter = tf
	st.mu.Unlock()
	h.flushTrace(st)
	return merged
}

// diffSkillNames returns names present in `before` but not in `after`,
// sorted for stable trace output.
func diffSkillNames(before, after []skills.Skill) []string {
	keep := make(map[string]bool, len(after))
	for _, s := range after {
		keep[s.Name()] = true
	}
	var out []string
	for _, s := range before {
		n := s.Name()
		if !keep[n] {
			out = append(out, n)
		}
	}
	return sortStrings(out)
}

func isIdentitySurface(s ToolSurface) bool {
	return len(s.Only) == 0 &&
		len(s.Include) == 0 &&
		len(s.Exclude) == 0 &&
		len(s.FromCapsules) == 0 &&
		s.MaxTools == 0
}

// SurfaceAuthoritative reports whether any cogni activated for this turn applies
// a non-identity ToolSurface. When true the host planner should treat the
// surfaced capability set as definitive — skipping its own per-message intent
// re-ranking and tool cap — so the tool block stays deterministic and
// prompt-cache friendly. Turn-cached: shares the same evaluation as
// BuildContext/FilterSkills, so it costs nothing extra within a turn.
func (h *Hook) SurfaceAuthoritative(req ContextRequest) bool {
	if h == nil {
		return false
	}
	st := h.evaluate(req)
	if st == nil {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, a := range st.activations {
		if a.Declaration != nil && !isIdentitySurface(a.Declaration.Surface) {
			return true
		}
	}
	return false
}

// ActiveIDs returns the IDs of every activated cogni for audit/UI purposes.
func (h *Hook) ActiveIDs(req ContextRequest) []string {
	acts := h.Activate(req)
	if len(acts) == 0 {
		return nil
	}
	ids := make([]string, 0, len(acts))
	for _, a := range acts {
		if a.Declaration != nil {
			ids = append(ids, a.Declaration.ID)
		}
	}
	return ids
}

// renderContextOnce produces the markdown block contributed by a single cogni.
// The header line is "### {DisplayName or ID}" so multiple cognis stack
// cleanly under the "## 智体上下文" parent heading. Returns (block, fellBack)
// where fellBack reports whether a Template error forced us to use Static.
func (h *Hook) renderContextOnce(d *Declaration, req ContextRequest) (string, bool) {
	if d == nil {
		return "", false
	}
	body := strings.TrimSpace(d.Context.Static)
	fellBack := false

	if tmplSrc := strings.TrimSpace(d.Context.Template); tmplSrc != "" {
		if rendered, err := h.execTemplate(d.ID, tmplSrc, req); err == nil {
			body = strings.TrimSpace(rendered)
		} else {
			fellBack = true
			slog.Warn("cogni: template render failed; falling back to Static",
				"id", d.ID, "err", err)
		}
	}

	// MemoryQuery: delegate to the host's memory recall primitive if attached.
	// Empty query or nil search function is a no-op; empty recall is also a
	// no-op (we won't inject a header with no content).
	if q := strings.TrimSpace(d.Context.MemoryQuery); q != "" {
		if fn := h.memorySearchFn(); fn != nil {
			query := strings.ReplaceAll(q, "{message}", req.Message)
			if recall := strings.TrimSpace(fn(context.Background(), req.TenantID, query)); recall != "" {
				if body != "" {
					body += "\n\n"
				}
				body += "#### 相关记忆\n" + recall
			}
		}
	}

	// Experience hints: inject relevant accumulated experience if available.
	if provider := h.experienceProviderFn(); provider != nil {
		if es := provider(d.ID); es != nil {
			if hints := es.ContextHints(context.Background(), req.Message); hints != "" {
				if body != "" {
					body += "\n\n"
				}
				body += hints
			}
		}
	}

	if body == "" {
		return "", fellBack
	}
	header := "### " + h.headingFor(d)
	return header + "\n" + body, fellBack
}

func (h *Hook) headingFor(d *Declaration) string {
	if d.DisplayName != "" {
		return d.DisplayName
	}
	return d.ID
}

func (h *Hook) execTemplate(id, src string, req ContextRequest) (string, error) {
	h.tmplMu.Lock()
	t, ok := h.tmpls[id+"|"+src]
	h.tmplMu.Unlock()
	if !ok {
		parsed, err := template.New(id).Parse(src)
		if err != nil {
			return "", err
		}
		h.tmplMu.Lock()
		h.tmpls[id+"|"+src] = parsed
		h.tmplMu.Unlock()
		t = parsed
	}
	data := map[string]any{
		"Message": req.Message,
		"Tenant":  req.TenantID,
		"Channel": req.Channel,
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
