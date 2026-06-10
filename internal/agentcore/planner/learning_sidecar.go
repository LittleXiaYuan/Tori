package planner

import (
	"context"
	"log/slog"
	"time"

	iledger "yunque-agent/internal/ledger"
	"yunque-agent/pkg/safego"
)

// MetaCogSidecar is the small Planner-facing interface for metacognitive
// anomaly handling. It keeps the concrete Ledger bridge outside the Planner
// core and makes the post-run/strategy-hook boundary explicit.
type MetaCogSidecar interface {
	CorrectionHint(taskID string) string
	ShouldEscalate(taskID string) bool
	FormatAnomalySummary(taskID string) string
	ClearTask(taskID string)
}

// TaskOutcomeSink receives the outcome of every planner run (success or
// failure) after the run completes. It is invoked on a background goroutine
// with a detached context, so implementations may do Ledger I/O or trigger
// heavier evolution work without delaying the user-facing reply.
type TaskOutcomeSink func(ctx context.Context, req PlanRequest, result *PlanResult, runErr error, reflectScore float64, elapsed time.Duration)

// LearningSidecar owns post-run learning and metacognition hooks that used to
// be direct Planner fields. Planner should execute tasks; learning side effects
// live here.
type LearningSidecar struct {
	dataCollector *DataCollector
	metacog       MetaCogSidecar
	outcomeSink   TaskOutcomeSink
}

func NewLearningSidecar() *LearningSidecar {
	return &LearningSidecar{}
}

func (s *LearningSidecar) SetDataCollector(dc *DataCollector) {
	s.dataCollector = dc
}

func (s *LearningSidecar) SetMetaCogBridge(b *iledger.MetaCogBridge) {
	s.metacog = b
}

func (s *LearningSidecar) SetMetaCogSidecar(m MetaCogSidecar) {
	s.metacog = m
}

// SetTaskOutcomeSink attaches a post-run outcome consumer (e.g. the
// evolution coordinator). At most one sink is supported.
func (s *LearningSidecar) SetTaskOutcomeSink(fn TaskOutcomeSink) {
	s.outcomeSink = fn
}

func (s *LearningSidecar) HasMetaCog() bool {
	return s != nil && s.metacog != nil
}

func (s *LearningSidecar) ShouldEscalate(taskID string) bool {
	return s != nil && s.metacog != nil && s.metacog.ShouldEscalate(taskID)
}

func (s *LearningSidecar) CorrectionHint(taskID string) string {
	if s == nil || s.metacog == nil {
		return ""
	}
	return s.metacog.CorrectionHint(taskID)
}

func (s *LearningSidecar) AfterRun(ctx context.Context, req PlanRequest, result *PlanResult, runErr error, reflect ReflectFunc, elapsed time.Duration) {
	if s == nil {
		return
	}
	// Reflect once and share the score between the data collector and the
	// outcome sink, so evolution decisions see the same quality signal that
	// gates training-data collection.
	var reflectScore float64
	if runErr == nil && result != nil && result.Reply != "" && reflect != nil &&
		(s.dataCollector != nil || s.outcomeSink != nil) {
		goal := extractGoal(req)
		if reflect(ctx, goal, result.Reply) {
			reflectScore = 0.8
		} else {
			reflectScore = 0.3
		}
	}
	if runErr == nil && s.dataCollector != nil && result != nil {
		s.dataCollector.Collect(ctx, req, result, reflectScore)
	}

	if sink := s.outcomeSink; sink != nil {
		safego.Go("learning-outcome-sink", func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			sink(bgCtx, req, result, runErr, reflectScore, elapsed)
		})
	}

	if s.metacog != nil && req.TaskID != "" {
		if summary := s.metacog.FormatAnomalySummary(req.TaskID); summary != "" {
			slog.Info("planner: metacog summary", "detail", summary)
		}
		s.metacog.ClearTask(req.TaskID)
	}
}
