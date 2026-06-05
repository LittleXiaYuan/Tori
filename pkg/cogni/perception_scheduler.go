package cogni

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// PerceptionScheduler manages cron-based and webhook-based perception triggers.
// It watches the Registry for declarations with schedule/webhook perception
// rules and fires activation events at the appropriate times.
type PerceptionScheduler struct {
	mu       sync.Mutex
	registry *Registry
	handler  PerceptionHandler
	crons    map[string]*cronEntry // keyed by "cogni:cron_expr"
	done     chan struct{}
	running  bool
}

type cronEntry struct {
	cogniID  string
	cron     string
	next     time.Time
	interval time.Duration
}

// PerceptionHandler is called when a perception event fires. The host
// provides this — it typically routes the activation through the planner.
type PerceptionHandler func(ctx context.Context, cogniID string, signal *PerceptionSignal)

func NewPerceptionScheduler(registry *Registry, handler PerceptionHandler) *PerceptionScheduler {
	return &PerceptionScheduler{
		registry: registry,
		handler:  handler,
		crons:    make(map[string]*cronEntry),
		done:     make(chan struct{}),
	}
}

// Start begins the scheduler loop. It scans for cron rules and fires them
// at the appropriate intervals. Call Stop() to shut down.
func (ps *PerceptionScheduler) Start() {
	ps.mu.Lock()
	if ps.running {
		ps.mu.Unlock()
		return
	}
	ps.running = true
	ps.mu.Unlock()

	ps.refresh()

	go ps.loop()
	slog.Info("cogni: perception scheduler started")
}

func (ps *PerceptionScheduler) Stop() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if !ps.running {
		return
	}
	ps.running = false
	close(ps.done)
	ps.done = make(chan struct{})
}

// Refresh rebuilds the cron table from the current declaration registry. It is
// safe to call while the scheduler is running or stopped.
func (ps *PerceptionScheduler) Refresh() {
	ps.refresh()
}

func (ps *PerceptionScheduler) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ps.done:
			return
		case now := <-ticker.C:
			ps.tick(now)
		}
	}
}

func (ps *PerceptionScheduler) tick(now time.Time) {
	ps.mu.Lock()
	var fired []cronEntry
	for key, entry := range ps.crons {
		if now.After(entry.next) {
			fired = append(fired, *entry)
			entry.next = now.Add(entry.interval)
			ps.crons[key] = entry
		}
	}
	ps.mu.Unlock()

	for _, e := range fired {
		if ps.handler != nil {
			signal := &PerceptionSignal{
				ScheduleTriggered: true,
				ScheduleCron:      e.cron,
			}
			ps.handler(context.Background(), e.cogniID, signal)
			slog.Debug("cogni: schedule fired", "cogni", e.cogniID, "cron", e.cron)
		}
	}
}

// refresh scans the registry for declarations with schedule perception rules.
func (ps *PerceptionScheduler) refresh() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.crons = make(map[string]*cronEntry)
	now := time.Now()

	for _, d := range ps.registry.Active() {
		for _, rule := range d.Activation.Perception {
			if rule.Type != "schedule" || rule.Cron == "" {
				continue
			}
			interval := parseCronInterval(rule.Cron)
			key := d.ID + ":" + rule.Cron
			ps.crons[key] = &cronEntry{
				cogniID:  d.ID,
				cron:     rule.Cron,
				next:     now.Add(interval),
				interval: interval,
			}
		}
	}

	if len(ps.crons) > 0 {
		slog.Info("cogni: schedule rules registered", "count", len(ps.crons))
	}
}

// HandleWebhook processes an incoming webhook and triggers matching cognis.
func (ps *PerceptionScheduler) HandleWebhook(ctx context.Context, path string, body map[string]any) {
	if ps.handler == nil {
		return
	}

	for _, d := range ps.registry.Active() {
		for _, rule := range d.Activation.Perception {
			if rule.Type != "webhook" || rule.Path != path {
				continue
			}
			signal := &PerceptionSignal{
				WebhookTriggered: true,
				WebhookPath:      path,
			}
			ps.handler(ctx, d.ID, signal)
			slog.Info("cogni: webhook triggered", "cogni", d.ID, "path", path)
		}
	}
}

// WebhookPaths returns all registered webhook paths for route registration.
func (ps *PerceptionScheduler) WebhookPaths() []string {
	seen := make(map[string]bool)
	var paths []string
	for _, d := range ps.registry.Active() {
		for _, rule := range d.Activation.Perception {
			if rule.Type == "webhook" && rule.Path != "" && !seen[rule.Path] {
				seen[rule.Path] = true
				paths = append(paths, rule.Path)
			}
		}
	}
	return paths
}

// parseCronInterval converts a simplified cron expression to a duration.
// Supports: "0 * * * *" (hourly), "0 2 * * *" (daily at 2am), etc.
// Falls back to 1 hour for complex/unparseable expressions.
func parseCronInterval(cron string) time.Duration {
	parts := strings.Fields(cron)
	if len(parts) < 5 {
		return time.Hour
	}

	min, hour, dom, _, _ := parts[0], parts[1], parts[2], parts[3], parts[4]

	switch {
	case min != "*" && hour == "*" && dom == "*":
		return time.Hour
	case min != "*" && hour != "*" && dom == "*":
		return 24 * time.Hour
	case min != "*" && hour != "*" && dom != "*":
		return 7 * 24 * time.Hour
	default:
		return time.Hour
	}
}
