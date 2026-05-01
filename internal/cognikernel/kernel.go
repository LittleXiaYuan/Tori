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

	running    bool
	subscribed bool // prevents duplicate event subscriptions on re-Start
	cancel     context.CancelFunc

	// Concurrency limiters: prevent goroutine leaks from rapid event bursts.
	reflectSem chan struct{} // limits concurrent reflective cycles
	dreamSem   chan struct{} // limits concurrent dreaming cycles
}

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
	return &CogniKernel{
		config:     cfg,
		eventBus:   NewEventBus(256),
		reflectSem: make(chan struct{}, 2), // max 2 concurrent reflections
		dreamSem:   make(chan struct{}, 1), // max 1 concurrent dream cycle
	}
}

func (k *CogniKernel) SetActiveLoop(al *ActiveLoop)       { k.active = al }
func (k *CogniKernel) SetReflectiveLoop(rl *ReflectiveLoop) { k.reflective = rl }
func (k *CogniKernel) SetDreamingLoop(dl *DreamingLoop)     { k.dreaming = dl }
func (k *CogniKernel) SetImmuneBridge(ib *ImmuneBridge)     { k.immune = ib }

// EventBus returns the kernel-wide event bus for external subscriptions.
func (k *CogniKernel) EventBus() *EventBus { return k.eventBus }

// Metrics returns a snapshot of kernel metrics.
func (k *CogniKernel) Metrics() KernelMetrics { return k.metrics.Snapshot() }

// Start begins background loops (reflective listener, dreaming scheduler).
// The active loop is driven externally by HandleConversation calls.
// Safe to call multiple times; subscriptions are registered only once.
func (k *CogniKernel) Start(ctx context.Context) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.running {
		return
	}
	k.running = true
	ctx, k.cancel = context.WithCancel(ctx)

	// Guard: register subscriptions only once to avoid duplicate handlers
	// across Stop/Start cycles.
	if !k.subscribed {
		k.subscribed = true

		if k.reflective != nil && k.config.ReflectAfterConversation {
			k.eventBus.Subscribe(EventConversationEnded, func(ev Event) {
				k.triggerReflection(ctx, ev)
			})
			slog.Info("cognikernel: reflective loop subscribed to conversation events")
		}

		if k.dreaming != nil && k.config.DreamingEnabled {
			k.eventBus.Subscribe(EventIdleDetected, func(ev Event) {
				k.triggerDreaming(ctx, ev)
			})
			slog.Info("cognikernel: dreaming loop subscribed to idle events")
		}

		if k.immune != nil && k.config.ImmuneEnabled {
			k.eventBus.Subscribe(EventSecurityAlert, func(ev Event) {
				k.immune.HandleEvent(ctx, ev)
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
func (k *CogniKernel) triggerReflection(ctx context.Context, ev Event) {
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

		reflectCtx, cancel := context.WithTimeout(ctx, k.config.ReflectTimeout)
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
func (k *CogniKernel) triggerDreaming(ctx context.Context, ev Event) {
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

		dreamCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
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
