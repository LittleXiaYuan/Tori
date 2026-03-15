package heartbeat

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskFunc is the function executed on each heartbeat tick.
// It receives a context and should return a result text and optional error.
type TaskFunc func(ctx context.Context) (string, error)

// ResultCallback is invoked after each heartbeat execution so the gateway
// (or another consumer) can decide how to deliver the result.
type ResultCallback func(log *Log, policy *DeliveryPolicy)

// ──────────────────────────────────────────────
// DeliveryPolicy — controls where heartbeat output goes
// ──────────────────────────────────────────────

// DeliveryTarget identifies the channel+session to deliver to.
type DeliveryTarget struct {
	Channel   string `json:"channel,omitempty"`   // e.g. "telegram", "feishu"
	SessionID string `json:"session_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
}

// SuppressRule defines conditions under which heartbeat output is silently dropped.
type SuppressRule struct {
	QuietHoursStart int  `json:"quiet_hours_start,omitempty"` // 0-23 hour
	QuietHoursEnd   int  `json:"quiet_hours_end,omitempty"`   // 0-23 hour
	OnlyOnError     bool `json:"only_on_error,omitempty"`     // suppress "ok" results
}

// DeliveryPolicy defines how and where heartbeat results are delivered.
type DeliveryPolicy struct {
	Targets  []DeliveryTarget `json:"targets,omitempty"`  // where to send
	Suppress SuppressRule     `json:"suppress,omitempty"` // when NOT to send
}

// IsSuppressed returns true if the policy's suppress rules say to drop this result.
func (dp *DeliveryPolicy) IsSuppressed(status string, now time.Time) bool {
	if dp == nil {
		return false
	}
	if dp.Suppress.OnlyOnError && status == "ok" {
		return true
	}
	h := now.Hour()
	start, end := dp.Suppress.QuietHoursStart, dp.Suppress.QuietHoursEnd
	if start != end {
		if start < end {
			if h >= start && h < end {
				return true
			}
		} else { // wraps midnight, e.g. 22-6
			if h >= start || h < end {
				return true
			}
		}
	}
	return false
}

// Log records one heartbeat execution.
type Log struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"` // "ok", "error"
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Duration    string     `json:"duration,omitempty"`
}

// Config defines heartbeat behavior.
type Config struct {
	Interval time.Duration // time between beats
	Enabled  bool
	MaxLogs  int // max log entries to keep
}

// Service manages periodic autonomous tasks.
type Service struct {
	mu       sync.Mutex
	config   Config
	task     TaskFunc
	onResult ResultCallback
	policy   *DeliveryPolicy
	logs     []Log
	cancel   context.CancelFunc
	running  bool
}

// New creates a heartbeat service.
func New(cfg Config, task TaskFunc) *Service {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Minute
	}
	if cfg.MaxLogs <= 0 {
		cfg.MaxLogs = 100
	}
	return &Service{
		config: cfg,
		task:   task,
		logs:   make([]Log, 0),
	}
}

// Start begins the heartbeat loop. No-op if already running or disabled.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running || !s.config.Enabled {
		return
	}

	hbCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true

	go s.loop(hbCtx)
	slog.Info("heartbeat started", "interval", s.config.Interval)
}

// Stop halts the heartbeat loop.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
	slog.Info("heartbeat stopped")
}

// IsRunning returns whether the heartbeat is active.
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// SetEnabled enables or disables heartbeat. Starts/stops as needed.
func (s *Service) SetEnabled(ctx context.Context, enabled bool) {
	s.config.Enabled = enabled
	if enabled {
		s.Start(ctx)
	} else {
		s.Stop()
	}
}

// SetOnResult registers a callback that fires after each heartbeat.
func (s *Service) SetOnResult(cb ResultCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onResult = cb
}

// SetDeliveryPolicy sets the delivery policy for heartbeat results.
func (s *Service) SetDeliveryPolicy(dp *DeliveryPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = dp
}

// Policy returns the current delivery policy (may be nil).
func (s *Service) Policy() *DeliveryPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.policy
}

// SetInterval changes the heartbeat interval. Restarts if running.
func (s *Service) SetInterval(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	wasRunning := s.IsRunning()
	if wasRunning {
		s.Stop()
	}
	s.mu.Lock()
	s.config.Interval = interval
	s.mu.Unlock()
	if wasRunning {
		s.Start(ctx)
	}
}

// Trigger manually executes one heartbeat.
func (s *Service) Trigger(ctx context.Context) *Log {
	return s.execute(ctx)
}

// Logs returns recent heartbeat logs (newest first).
func (s *Service) Logs(limit int) []Log {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit > len(s.logs) {
		limit = len(s.logs)
	}
	result := make([]Log, limit)
	// Return newest first
	for i := 0; i < limit; i++ {
		result[i] = s.logs[len(s.logs)-1-i]
	}
	return result
}

// ClearLogs removes all heartbeat logs.
func (s *Service) ClearLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = s.logs[:0]
}

func (s *Service) loop(ctx context.Context) {
	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.execute(ctx)
		}
	}
}

func (s *Service) execute(ctx context.Context) *Log {
	if s.task == nil {
		return nil
	}

	entry := Log{
		ID:        uuid.New().String(),
		StartedAt: time.Now(),
	}

	result, err := s.task(ctx)
	now := time.Now()
	entry.CompletedAt = &now
	entry.Duration = now.Sub(entry.StartedAt).String()

	if err != nil {
		entry.Status = "error"
		entry.Error = err.Error()
		slog.Warn("heartbeat error", "err", err)
	} else {
		entry.Status = "ok"
		entry.Result = result
		slog.Info("heartbeat ok", "duration", entry.Duration)
	}

	s.mu.Lock()
	s.logs = append(s.logs, entry)
	// Trim logs
	if len(s.logs) > s.config.MaxLogs {
		s.logs = s.logs[len(s.logs)-s.config.MaxLogs:]
	}
	cb := s.onResult
	pol := s.policy
	s.mu.Unlock()

	// Fire delivery callback (skip if suppressed by policy)
	if cb != nil && (pol == nil || !pol.IsSuppressed(entry.Status, time.Now())) {
		cb(&entry, pol)
	}

	return &entry
}
