package cogni

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// Alert is one observation produced by a Sentinel scan. Alerts are stable
// across scans: the same (CogniID, Kind) tuple keeps its `Since` timestamp
// while the condition holds, so operators can filter for alerts that have
// been firing for longer than X.
type Alert struct {
	CogniID       string    `json:"cogni_id"`
	Kind          AlertKind `json:"kind"`
	Severity      string    `json:"severity"` // "info" | "warn" | "critical"
	Message       string    `json:"message"`
	Score         int       `json:"score"`
	Since         time.Time `json:"since"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	// AutoActionTaken is set when the Sentinel's automatic mitigation
	// policy (e.g. auto-disable on critical) has already acted on this
	// alert so the UI doesn't offer the same action twice.
	AutoActionTaken string `json:"auto_action_taken,omitempty"`
}

// AlertKind enumerates the rule that fired. Keeping this a string rather
// than an int lets us evolve rules without migration churn.
type AlertKind string

const (
	AlertUnhealthy             AlertKind = "unhealthy_score"
	AlertChronicSuppression    AlertKind = "chronic_suppression"
	AlertTemplateErrors        AlertKind = "template_errors"
	AlertSurfaceFallback       AlertKind = "surface_fallback"
	AlertDeclarationChecksFail AlertKind = "declaration_checks_failed"
)

// SentinelPolicy configures a Sentinel's scanning cadence and
// automatic-action behaviour.
type SentinelPolicy struct {
	// Interval between background scans. <=0 disables background scanning;
	// callers can still invoke Scan() directly.
	Interval time.Duration

	// WindowSize caps the number of recent traces each scan inspects.
	// 0 uses the TraceStore default (~256).
	WindowSize int

	// AutoDisableOnCritical is OFF by default. Enable only when the
	// operator wants the agent to self-heal by disabling unhealthy cognis
	// automatically. Even then, the action is logged + alerts are retained
	// so the operator can see + re-enable.
	AutoDisableOnCritical bool

	// MinEvaluations protects against over-reaction on thin data.
	// Alerts are suppressed until the cogni has at least this many
	// evaluations in the window. Default 5.
	MinEvaluations int
}

// Sentinel runs periodic health checks against a TraceStore and emits
// structured Alerts. It optionally disables cognis whose score stays in
// the critical range for too long.
//
// Sentinel is goroutine-safe. Start returns immediately; Stop is
// idempotent and safely joinable.
type Sentinel struct {
	store    TraceStore
	registry *Registry
	policy   SentinelPolicy

	mu     sync.RWMutex
	alerts map[string]Alert // key = "<cogni_id>|<kind>"

	stopCh chan struct{}
	doneCh chan struct{}
	once   sync.Once
}

// NewSentinel wires a Sentinel around the given store and registry.
// Passing nil for either produces a Sentinel whose Scan/Start is a no-op,
// keeping call sites boring.
func NewSentinel(store TraceStore, registry *Registry, policy SentinelPolicy) *Sentinel {
	if policy.MinEvaluations <= 0 {
		policy.MinEvaluations = 5
	}
	return &Sentinel{
		store:    store,
		registry: registry,
		policy:   policy,
		alerts:   make(map[string]Alert),
	}
}

// Start begins the background scan loop. It is safe to call multiple
// times; subsequent calls are ignored.
func (s *Sentinel) Start(ctx context.Context) {
	if s == nil || s.store == nil {
		return
	}
	if s.policy.Interval <= 0 {
		return
	}
	s.once.Do(func() {
		s.stopCh = make(chan struct{})
		s.doneCh = make(chan struct{})
		go s.loop(ctx)
	})
}

// Stop halts the background loop if running.  Safe to call without Start.
func (s *Sentinel) Stop() {
	if s == nil || s.stopCh == nil {
		return
	}
	select {
	case <-s.stopCh:
		return // already stopped
	default:
	}
	close(s.stopCh)
	<-s.doneCh
}

// Scan runs one evaluation pass over the recent trace window, updates the
// internal alert table, and (when AutoDisableOnCritical is true) disables
// cognis whose score fell to the unhealthy band. Returns the active alert
// set after the scan, sorted by (severity desc, id asc).
//
// Three rule families run per scan:
//   1. Runtime SLO rules fed by the TraceStore (unhealthy score, chronic
//      suppression, persistent template errors).
//   2. Declarative Check failures drawn from Registry.VerifyAll — treats a
//      broken assertion as a config-level regression.
//   3. Recovery: any alert key no longer matched is evicted so stale
//      conditions don't haunt the UI.
func (s *Sentinel) Scan() []Alert {
	if s == nil || s.store == nil {
		return nil
	}
	mon := NewMonitor(s.store)
	metrics := mon.ComputeAll(s.policy.WindowSize)

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	currentKeys := make(map[string]bool)
	commit := func(fired Alert) {
		key := fired.CogniID + "|" + string(fired.Kind)
		currentKeys[key] = true
		if existing, ok := s.alerts[key]; ok {
			fired.Since = existing.Since
			fired.AutoActionTaken = existing.AutoActionTaken
		} else {
			fired.Since = now
		}
		fired.LastCheckedAt = now

		if s.policy.AutoDisableOnCritical && fired.Severity == "critical" && fired.AutoActionTaken == "" && s.registry != nil {
			if err := s.registry.SetEnabled(fired.CogniID, false); err == nil {
				fired.AutoActionTaken = "disabled"
				slog.Warn("cogni.sentinel: auto-disabled unhealthy cogni",
					"id", fired.CogniID, "kind", fired.Kind, "score", fired.Score)
			} else {
				slog.Warn("cogni.sentinel: auto-disable failed",
					"id", fired.CogniID, "err", err)
			}
		}
		s.alerts[key] = fired
	}

	for _, m := range metrics {
		for _, fired := range s.evaluate(m) {
			commit(fired)
		}
	}

	// Declarative check failures — these fire regardless of runtime
	// activity so even "cold" misconfigured cognis surface immediately.
	if s.registry != nil {
		for _, fired := range s.scanCheckFailures() {
			commit(fired)
		}
	}

	// Garbage-collect alerts whose condition recovered.
	for key := range s.alerts {
		if !currentKeys[key] {
			delete(s.alerts, key)
		}
	}

	return s.snapshotLocked()
}

// scanCheckFailures produces one Alert per cogni with ≥1 failing declarative
// check. The message lists up to 3 failing check labels so the UI has
// enough to act on without loading the full VerifyAll payload.
func (s *Sentinel) scanCheckFailures() []Alert {
	results := s.registry.VerifyAll()
	out := make([]Alert, 0, len(results))
	for id, checks := range results {
		var failed []CheckResult
		for _, c := range checks {
			if !c.Passed && c.Reason != "no assertion configured (ignored)" {
				failed = append(failed, c)
			}
		}
		if len(failed) == 0 {
			continue
		}
		labels := make([]string, 0, 3)
		for i, c := range failed {
			if i >= 3 {
				break
			}
			label := c.CheckName
			if label == "" {
				label = fmt.Sprintf("check[%d]", c.CheckIndex)
			}
			labels = append(labels, label)
		}
		more := ""
		if len(failed) > 3 {
			more = fmt.Sprintf(" (+%d more)", len(failed)-3)
		}
		out = append(out, Alert{
			CogniID:  id,
			Kind:     AlertDeclarationChecksFail,
			Severity: "critical",
			Message:  fmt.Sprintf("%d declarative check(s) failing: %s%s", len(failed), strings.Join(labels, ", "), more),
		})
	}
	return out
}

// Alerts returns the current set without running a scan, sorted for UI stability.
func (s *Sentinel) Alerts() []Alert {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

func (s *Sentinel) snapshotLocked() []Alert {
	out := make([]Alert, 0, len(s.alerts))
	for _, a := range s.alerts {
		out = append(out, a)
	}
	order := map[string]int{"critical": 0, "warn": 1, "info": 2}
	sort.Slice(out, func(i, j int) bool {
		oi := order[out[i].Severity]
		oj := order[out[j].Severity]
		if oi != oj {
			return oi < oj
		}
		return out[i].CogniID < out[j].CogniID
	})
	return out
}

// evaluate applies each rule to a single cogni's metrics.
func (s *Sentinel) evaluate(m HealthMetrics) []Alert {
	var out []Alert
	if m.Evaluations < s.policy.MinEvaluations {
		return nil
	}

	// Rule 1: overall score in critical band
	if m.Status == "unhealthy" {
		out = append(out, Alert{
			CogniID:  m.ID,
			Kind:     AlertUnhealthy,
			Severity: "critical",
			Score:    m.Score,
			Message:  fmt.Sprintf("score %d (%d activations / %d evaluations)", m.Score, m.Activations, m.Evaluations),
		})
	} else if m.Status == "warn" {
		out = append(out, Alert{
			CogniID:  m.ID,
			Kind:     AlertUnhealthy,
			Severity: "warn",
			Score:    m.Score,
			Message:  fmt.Sprintf("score %d drifting toward unhealthy", m.Score),
		})
	}

	// Rule 2: chronic suppression — the cogni can never win exclusivity
	if m.Activations == 0 && m.SuppressionRate > 0.8 && m.Suppressed >= s.policy.MinEvaluations {
		out = append(out, Alert{
			CogniID:  m.ID,
			Kind:     AlertChronicSuppression,
			Severity: "warn",
			Score:    m.Score,
			Message:  fmt.Sprintf("suppressed in %.0f%% of evaluations; likely wrong exclusive group or priority", m.SuppressionRate*100),
		})
	}

	// Rule 3: template fallback rate > 10% indicates a bug in the declaration
	if m.TemplateFallbackRate > 0.1 && m.Activations >= s.policy.MinEvaluations {
		sev := "warn"
		if m.TemplateFallbackRate > 0.5 {
			sev = "critical"
		}
		out = append(out, Alert{
			CogniID:  m.ID,
			Kind:     AlertTemplateErrors,
			Severity: sev,
			Score:    m.Score,
			Message:  fmt.Sprintf("template render failed in %.0f%% of activations", m.TemplateFallbackRate*100),
		})
	}

	return out
}

func (s *Sentinel) loop(ctx context.Context) {
	defer close(s.doneCh)
	t := time.NewTicker(s.policy.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-t.C:
			s.Scan()
		}
	}
}
