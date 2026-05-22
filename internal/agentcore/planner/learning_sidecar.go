package planner

import (
	"context"
	"log/slog"

	iledger "yunque-agent/internal/ledger"
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

// LearningSidecar owns post-run learning and metacognition hooks that used to
// be direct Planner fields. Planner should execute tasks; learning side effects
// live here.
type LearningSidecar struct {
	dataCollector *DataCollector
	metacog       MetaCogSidecar
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

func (s *LearningSidecar) AfterRun(ctx context.Context, req PlanRequest, result *PlanResult, runErr error, reflect ReflectFunc) {
	if s == nil {
		return
	}
	if runErr == nil && s.dataCollector != nil && result != nil {
		var reflectScore float64
		if reflect != nil && result.Reply != "" {
			goal := extractGoal(req)
			if reflect(ctx, goal, result.Reply) {
				reflectScore = 0.8
			} else {
				reflectScore = 0.3
			}
		}
		s.dataCollector.Collect(ctx, req, result, reflectScore)
	}

	if s.metacog != nil && req.TaskID != "" {
		if summary := s.metacog.FormatAnomalySummary(req.TaskID); summary != "" {
			slog.Info("planner: metacog summary", "detail", summary)
		}
		s.metacog.ClearTask(req.TaskID)
	}
}
