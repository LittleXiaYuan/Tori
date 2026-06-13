package cognikernel

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// CogniKernel orchestrates the three cognitive loops:
//   - ActiveLoop:     user-driven perceive→reason→act (delegates to Planner)
//   - ReflectiveLoop: async reflect→learn→distill after each conversation
//   - DreamingLoop:   idle/scheduled reverie→curiosity→skill growth
//
// It also bridges the ImmuneSystem (Trust+Guardrails+CircuitBreaker) as a
// cross-cutting safety layer across all loops.
type CogniKernel struct {
	mu sync.RWMutex

	active     *ActiveLoop
	reflective *ReflectiveLoop
	dreaming   *DreamingLoop
	immune     *ImmuneBridge

	eventBus *EventBus
	config   KernelConfig
	metrics  KernelMetrics

	running bool
	cancel  context.CancelFunc

	// ctxMu protects currentCtx so handlers always use the latest Start's context.
	ctxMu      sync.RWMutex
	currentCtx context.Context

	// stoppedCtx is a pre-cancelled context returned by ctx() after Stop().
	stoppedCtx context.Context

	// Concurrency limiters: prevent goroutine leaks from rapid event bursts.
	reflectSem chan struct{} // limits concurrent reflective cycles
	dreamSem   chan struct{} // limits concurrent dreaming cycles

	// Offline self-evolution (小羽 / RWKV-7). dreamDistill runs one stateful
	// dream cycle on the local engine and returns distilled experiences;
	// experienceSink persists them to the durable truth source so the next day's
	// front-stage recall surfaces them. dreamSessionID carries the RWKV O(1)
	// recurrent-state handle across cycles. All guarded by mu.
	dreamDistill   DreamDistillFunc
	experienceSink ExperienceSinkFunc
	dreamSessionID string

	// sessionLoad/sessionSave persist the RWKV state handle across process
	// restarts (Ledger KV), so O(1) state continuity is durable, not just
	// in-memory. Optional; nil means in-process continuity only.
	sessionLoad SessionLoadFunc
	sessionSave SessionSaveFunc

	// dreamEventEmit publishes a per-cycle digest to the durable event stream
	// (Ledger), so the offline dream loop is observable (e.g. on the Inner-life
	// timeline) without coupling the kernel to the gateway.
	dreamEventEmit EventEmitFunc

	// Observability snapshot of the last completed offline dream cycle.
	lastDreamAt          time.Time
	lastDreamExperiences int

	// enabledFn, when set, lets the kernel honor a runtime hot-toggle of the
	// cognitive layer: each dream cycle is skipped while it returns false, so
	// flipping the master switch live also quiets the background dreaming loop
	// (no restart). nil means always-on.
	enabledFn func() bool
}

// DreamStatusSnapshot is a read-only view of the offline dream loop's recent
// activity, for observability surfaces.
type DreamStatusSnapshot struct {
	Running              bool      `json:"running"`
	DreamCycles          int64     `json:"dream_cycles"`
	ExperiencesAdded     int64     `json:"experiences_added"`
	LastDreamAt          time.Time `json:"last_dream_at,omitempty"`
	LastDreamExperiences int       `json:"last_dream_experiences"`
	SessionID            string    `json:"session_id,omitempty"`
}

// SessionLoadFunc returns the last persisted RWKV state handle, or "" if none.
type SessionLoadFunc func(ctx context.Context) string

// SessionSaveFunc persists the latest RWKV state handle for next-restart reuse.
type SessionSaveFunc func(ctx context.Context, sessionID string)

// DreamExperience is a distilled lesson produced by an offline dream cycle.
// It is a decoupled mirror of the durable experience record so cognikernel does
// not import the offline driver or the experience store directly.
type DreamExperience struct {
	Category string
	Outcome  string
	Lesson   string
	Context  string
	Tags     []string
}

// DreamDistillFunc runs one offline (小羽 / RWKV-7) dream cycle. It receives the
// previous RWKV state handle (prevSessionID) for O(1) continuity and returns the
// new handle plus any distilled experiences.
type DreamDistillFunc func(ctx context.Context, prevSessionID string) (newSessionID string, experiences []DreamExperience, err error)

// ExperienceSinkFunc persists a distilled experience into the durable truth
// source (Ledger experience store / experience.json).
type ExperienceSinkFunc func(ctx context.Context, exp DreamExperience) error

// KernelConfig controls the behavior of the cognitive kernel.
type KernelConfig struct {
	// ReflectAfterConversation triggers the reflective loop after each
	// conversation end. Default: true.
	ReflectAfterConversation bool `json:"reflect_after_conversation"`

	// DreamingEnabled allows the dreaming loop to run during idle. Default: true.
	DreamingEnabled bool `json:"dreaming_enabled"`

	// ImmuneEnabled enables the immune bridge safety checks. Default: true.
	ImmuneEnabled bool `json:"immune_enabled"`

	// MinIdleBeforeDream is the minimum idle duration before triggering a
	// dreaming cycle. Default: 15 minutes.
	MinIdleBeforeDream time.Duration `json:"min_idle_before_dream"`

	// ReflectTimeout caps how long a reflective cycle can run. Default: 30s.
	ReflectTimeout time.Duration `json:"reflect_timeout"`
}

func DefaultKernelConfig() KernelConfig {
	return KernelConfig{
		ReflectAfterConversation: true,
		DreamingEnabled:          true,
		ImmuneEnabled:            true,
		MinIdleBeforeDream:       15 * time.Minute,
		ReflectTimeout:           30 * time.Second,
	}
}

// KernelMetrics tracks operational statistics across all loops.
type KernelMetrics struct {
	mu sync.Mutex

	ActiveCycles     int64   `json:"active_cycles"`
	ReflectCycles    int64   `json:"reflect_cycles"`
	DreamCycles      int64   `json:"dream_cycles"`
	ImmuneCatches    int64   `json:"immune_catches"`
	AvgReflectScore  float64 `json:"avg_reflect_score"`
	ExperiencesAdded int64   `json:"experiences_added"`
	SkillsGrown      int64   `json:"skills_grown"`
}

func (m *KernelMetrics) recordActive() {
	m.mu.Lock()
	m.ActiveCycles++
	m.mu.Unlock()
}

func (m *KernelMetrics) recordReflect(score float64) {
	m.mu.Lock()
	m.ReflectCycles++
	total := float64(m.ReflectCycles)
	m.AvgReflectScore = m.AvgReflectScore*(total-1)/total + score/total
	m.mu.Unlock()
}

func (m *KernelMetrics) recordDream() {
	m.mu.Lock()
	m.DreamCycles++
	m.mu.Unlock()
}

func (m *KernelMetrics) recordExperience() {
	m.mu.Lock()
	m.ExperiencesAdded++
	m.mu.Unlock()
}

func (m *KernelMetrics) Snapshot() KernelMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	return KernelMetrics{
		ActiveCycles:     m.ActiveCycles,
		ReflectCycles:    m.ReflectCycles,
		DreamCycles:      m.DreamCycles,
		ImmuneCatches:    m.ImmuneCatches,
		AvgReflectScore:  m.AvgReflectScore,
		ExperiencesAdded: m.ExperiencesAdded,
		SkillsGrown:      m.SkillsGrown,
	}
}

// New creates a CogniKernel with the given configuration.
// All loop components must be set via Set* methods before Start().
func New(cfg KernelConfig) *CogniKernel {
	stopped, stopFn := context.WithCancel(context.Background())
	stopFn()
	return &CogniKernel{
		config:     cfg,
		eventBus:   NewEventBus(256),
		stoppedCtx: stopped,
		reflectSem: make(chan struct{}, 2), // max 2 concurrent reflections
		dreamSem:   make(chan struct{}, 1), // max 1 concurrent dream cycle
	}
}

func (k *CogniKernel) SetActiveLoop(al *ActiveLoop)        { k.active = al }
func (k *CogniKernel) SetReflectiveLoop(rl *ReflectiveLoop) { k.reflective = rl }
func (k *CogniKernel) SetDreamingLoop(dl *DreamingLoop)     { k.dreaming = dl }
func (k *CogniKernel) SetImmuneBridge(ib *ImmuneBridge)     { k.immune = ib }

// SetDreamDistill wires the offline (小羽 / RWKV-7) dream driver.
func (k *CogniKernel) SetDreamDistill(fn DreamDistillFunc) { k.dreamDistill = fn }

// SetExperienceSink wires the durable experience persister (anti-fragmentation:
// it must point at the same store the planner reads for strategy injection).
func (k *CogniKernel) SetExperienceSink(fn ExperienceSinkFunc) { k.experienceSink = fn }

// SetSessionStore wires durable persistence of the RWKV state handle so O(1)
// continuity survives process restarts. Either hook may be nil.
func (k *CogniKernel) SetSessionStore(load SessionLoadFunc, save SessionSaveFunc) {
	k.sessionLoad = load
	k.sessionSave = save
}

// SetDreamEventEmit wires a durable digest emitter (e.g. Ledger events) called
// once per completed offline dream cycle, for observability.
func (k *CogniKernel) SetDreamEventEmit(fn EventEmitFunc) { k.dreamEventEmit = fn }

// SetEnabledCheck wires a runtime predicate (e.g. the cognitive-layer master
// switch) consulted before each dream cycle, so a live hot-toggle also stops/
// resumes the background dreaming loop without a restart.
func (k *CogniKernel) SetEnabledCheck(fn func() bool) { k.enabledFn = fn }

// DreamStatus returns a snapshot of recent offline dream activity. Pure read,
// safe for HTTP/diagnostic surfaces.
func (k *CogniKernel) DreamStatus() DreamStatusSnapshot {
	m := k.metrics.Snapshot()
	k.mu.RLock()
	defer k.mu.RUnlock()
	return DreamStatusSnapshot{
		Running:              k.running,
		DreamCycles:          m.DreamCycles,
		ExperiencesAdded:     m.ExperiencesAdded,
		LastDreamAt:          k.lastDreamAt,
		LastDreamExperiences: k.lastDreamExperiences,
		SessionID:            k.dreamSessionID,
	}
}

// ctx returns the current lifecycle context, safe for use in event handlers.
// Returns a pre-cancelled context if the kernel is stopped, ensuring any
// in-flight handler sees ctx.Err() != nil and exits promptly.
func (k *CogniKernel) ctx() context.Context {
	k.ctxMu.RLock()
	defer k.ctxMu.RUnlock()
	if k.currentCtx == nil {
		return k.stoppedCtx
	}
	return k.currentCtx
}

// EventBus returns the kernel-wide event bus for external subscriptions.
func (k *CogniKernel) EventBus() *EventBus { return k.eventBus }

// Metrics returns a snapshot of kernel metrics.
func (k *CogniKernel) Metrics() KernelMetrics { return k.metrics.Snapshot() }

// Start begins background loops (reflective listener, dreaming scheduler).
// The active loop is driven externally by HandleConversation calls.
// Safe to call multiple times; subscriptions are registered only once.
// Handlers always read the latest lifecycle context via k.ctx(), so
// Stop+Start cycles correctly use the new context.
func (k *CogniKernel) Start(ctx context.Context) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.running {
		return
	}
	k.running = true

	childCtx, cancel := context.WithCancel(ctx)
	k.cancel = cancel

	k.ctxMu.Lock()
	k.currentCtx = childCtx
	k.ctxMu.Unlock()

	// Register subscriptions once; handlers call k.ctx() each invocation
	// so they always use the latest Start's context.
	if !k.eventBus.hasSubscribers(EventConversationEnded) {
		if k.reflective != nil && k.config.ReflectAfterConversation {
			k.eventBus.Subscribe(EventConversationEnded, func(ev Event) {
				k.triggerReflection(ev)
			})
			slog.Info("cognikernel: reflective loop subscribed to conversation events")
		}
	}

	if !k.eventBus.hasSubscribers(EventIdleDetected) {
		if k.dreaming != nil && k.config.DreamingEnabled {
			k.eventBus.Subscribe(EventIdleDetected, func(ev Event) {
				k.triggerDreaming(ev)
			})
			slog.Info("cognikernel: dreaming loop subscribed to idle events")
		}
	}

	if !k.eventBus.hasSubscribers(EventSecurityAlert) {
		if k.immune != nil && k.config.ImmuneEnabled {
			k.eventBus.Subscribe(EventSecurityAlert, func(ev Event) {
				k.immune.HandleEvent(k.ctx(), ev)
			})
			slog.Info("cognikernel: immune bridge subscribed to security events")
		}
	}

	slog.Info("cognikernel: started",
		"reflect_after_conv", k.config.ReflectAfterConversation,
		"dreaming", k.config.DreamingEnabled,
		"immune", k.config.ImmuneEnabled,
	)
}

// Stop shuts down all background loops.
func (k *CogniKernel) Stop() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.running {
		return
	}
	if k.cancel != nil {
		k.cancel()
	}
	k.ctxMu.Lock()
	k.currentCtx = nil
	k.ctxMu.Unlock()

	k.running = false
	slog.Info("cognikernel: stopped")
}

// OnConversationEnd should be called after each conversation round completes.
// It publishes a ConversationEnded event which triggers the reflective loop.
func (k *CogniKernel) OnConversationEnd(data ConversationEndData) {
	k.metrics.recordActive()
	k.eventBus.Publish(Event{
		Type:      EventConversationEnded,
		Timestamp: time.Now(),
		Data:      data,
	})
}

// OnIdle should be called when the agent detects an idle period.
func (k *CogniKernel) OnIdle(tenantID string) {
	k.eventBus.Publish(Event{
		Type:      EventIdleDetected,
		Timestamp: time.Now(),
		Data:      IdleData{TenantID: tenantID},
	})
}

// triggerReflection runs the reflective loop asynchronously.
// Uses a semaphore to prevent unbounded goroutine growth from event bursts.
func (k *CogniKernel) triggerReflection(ev Event) {
	data, ok := ev.Data.(ConversationEndData)
	if !ok {
		slog.Warn("cognikernel: invalid ConversationEndData")
		return
	}

	// Non-blocking semaphore acquire: drop if at capacity
	select {
	case k.reflectSem <- struct{}{}:
	default:
		slog.Warn("cognikernel: reflective loop at capacity, skipping")
		return
	}

	go func() {
		defer func() { <-k.reflectSem }()

		// Always read the latest lifecycle context
		reflectCtx, cancel := context.WithTimeout(k.ctx(), k.config.ReflectTimeout)
		defer cancel()

		result, err := k.reflective.Run(reflectCtx, data)
		if err != nil {
			slog.Warn("cognikernel: reflective loop failed", "err", err)
			return
		}

		k.metrics.recordReflect(float64(result.Quality))
		if result.ExperiencesAdded > 0 {
			for i := 0; i < result.ExperiencesAdded; i++ {
				k.metrics.recordExperience()
			}
		}

		slog.Info("cognikernel: reflective loop completed",
			"quality", result.Quality,
			"satisfied", result.Satisfied,
			"experiences", result.ExperiencesAdded,
			"distilled", result.DistilledRules,
		)
	}()
}

// triggerDreaming runs the dreaming loop asynchronously.
// Uses a semaphore (capacity 1) to ensure only one dream cycle at a time.
func (k *CogniKernel) triggerDreaming(ev Event) {
	data, ok := ev.Data.(IdleData)
	if !ok {
		return
	}

	select {
	case k.dreamSem <- struct{}{}:
	default:
		slog.Debug("cognikernel: dreaming loop already running, skipping")
		return
	}

	go func() {
		defer func() { <-k.dreamSem }()

		dreamCtx, cancel := context.WithTimeout(k.ctx(), 5*time.Minute)
		defer cancel()

		result, err := k.dreaming.Run(dreamCtx, data.TenantID)
		if err != nil {
			slog.Warn("cognikernel: dreaming loop failed", "err", err)
			return
		}

		k.metrics.recordDream()
		slog.Info("cognikernel: dreaming loop completed",
			"thoughts", result.ThoughtsGenerated,
			"explorations", result.ExplorationsRun,
			"skills_suggested", result.SkillsSuggested,
		)
	}()
}

// StartDreamingScheduler launches a boot-time ticker that drives the dreaming +
// offline self-evolution cycle at a fixed interval. Unlike the night-only
// scheduler, this keeps the agent "dreaming" on the local engine (小羽 / RWKV-7)
// at zero cloud cost whenever the interval elapses. Safe to call once; the
// goroutine exits when ctx is cancelled.
func (k *CogniKernel) StartDreamingScheduler(ctx context.Context, interval time.Duration, tenantID string) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	// Restore the persisted RWKV state handle so the first cycle continues from
	// where the last process run left off (durable O(1) continuity).
	if k.sessionLoad != nil {
		if handle := k.sessionLoad(ctx); handle != "" {
			k.mu.Lock()
			k.dreamSessionID = handle
			k.mu.Unlock()
			slog.Info("cognikernel: restored dream session handle", "session", handle)
		}
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				k.runDreamCycle(ctx, tenantID)
			}
		}
	}()
	slog.Info("cognikernel: dreaming scheduler started", "interval", interval, "tenant", tenantID)
}

// runDreamCycle executes one scheduled dream: first the curiosity/reverie
// DreamingLoop (if wired), then the offline self-evolution pass that drives
// 小羽 (RWKV-7) to distill experiences and sinks them into the durable truth
// source. Concurrency is capped at one via dreamSem.
func (k *CogniKernel) runDreamCycle(ctx context.Context, tenantID string) {
	// Honor a live hot-toggle of the cognitive layer.
	if k.enabledFn != nil && !k.enabledFn() {
		return
	}
	select {
	case k.dreamSem <- struct{}{}:
	default:
		slog.Debug("cognikernel: dream cycle already running, skipping")
		return
	}
	defer func() { <-k.dreamSem }()

	// Phase 1: legacy curiosity → memory + reverie dreaming loop.
	if k.dreaming != nil {
		dctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		if _, err := k.dreaming.Run(dctx, tenantID); err != nil {
			slog.Warn("cognikernel: dreaming loop run failed", "err", err)
		}
		cancel()
	}

	// Phase 2: offline self-evolution. Drive the local RWKV-7 engine to chew on
	// recent failures and distill experiences, persisting them via the sink.
	if k.dreamDistill == nil {
		return
	}
	// Long timeout: the local model is latency-tolerant by design.
	dctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	k.mu.RLock()
	prevSession := k.dreamSessionID
	k.mu.RUnlock()

	newSession, experiences, err := k.dreamDistill(dctx, prevSession)
	if err != nil {
		slog.Warn("cognikernel: offline dream distill failed", "err", err)
		return
	}
	if newSession != "" {
		// Persist the RWKV O(1) state handle for next-cycle continuity (memory),
		// and durably (Ledger KV) so it also survives a process restart.
		k.mu.Lock()
		k.dreamSessionID = newSession
		k.mu.Unlock()
		if k.sessionSave != nil {
			k.sessionSave(dctx, newSession)
		}
	}

	k.metrics.recordDream()
	for _, exp := range experiences {
		if k.experienceSink == nil {
			break
		}
		if err := k.experienceSink(dctx, exp); err != nil {
			slog.Warn("cognikernel: experience sink failed", "err", err)
			continue
		}
		k.metrics.recordExperience()
	}

	// Observability: record the snapshot and emit a durable digest event so the
	// dream loop is visible (Inner-life timeline / status surfaces).
	k.mu.Lock()
	k.lastDreamAt = time.Now()
	k.lastDreamExperiences = len(experiences)
	k.mu.Unlock()
	if k.dreamEventEmit != nil {
		// Emit the standard "dreaming.completed" kind so the offline cycle shows
		// up in the inner-life Pack timeline alongside curiosity dreams; the
		// phase/engine fields distinguish it as the RWKV self-evolution pass.
		k.dreamEventEmit(dctx, "dreaming.completed", map[string]any{
			"tenant_id":   tenantID,
			"phase":       "offline_evolution",
			"engine":      "xiaoyu",
			"experiences": len(experiences),
			"session_id":  newSession,
		})
	}

	slog.Info("cognikernel: offline dream cycle complete",
		"experiences", len(experiences),
		"session", newSession,
	)
}
