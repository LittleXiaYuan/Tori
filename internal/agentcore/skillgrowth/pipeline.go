package skillgrowth

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Stage names the canonical skill-growth lifecycle stages.
type Stage string

const (
	StageDetect   Stage = "detect"
	StageGenerate Stage = "generate"
	StageReview   Stage = "review"
	StagePromote  Stage = "promote"
	StageObserve  Stage = "observe"
	StageRollback Stage = "rollback"
)

// Event is an append-only description of what happened in the pipeline.
// Callers can persist or expose these events without depending on any concrete
// detector, generator, or lifecycle implementation.
type Event struct {
	Stage        Stage     `json:"stage"`
	CapabilityID string    `json:"capability_id,omitempty"`
	CandidateID  string    `json:"candidate_id,omitempty"`
	Outcome      string    `json:"outcome,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	At           time.Time `json:"at"`
}

// Gap describes a missing or repeated capability discovered by a detector.
type Gap struct {
	CapabilityID   string            `json:"capability_id"`
	Description    string            `json:"description"`
	FailureContext string            `json:"failure_context,omitempty"`
	Source         string            `json:"source,omitempty"`
	Evidence       map[string]string `json:"evidence,omitempty"`
	DetectedAt     time.Time         `json:"detected_at"`
}

// DescriptionWithContext returns the generation prompt used by candidate
// generators that need a single reason string.
func (g Gap) DescriptionWithContext() string {
	if g.FailureContext == "" {
		return g.Description
	}
	if g.Description == "" {
		return g.FailureContext
	}
	return g.Description + "\nContext: " + g.FailureContext
}

// Candidate describes a generated ability before it is promoted into runtime.
type Candidate struct {
	ID           string            `json:"id"`
	CapabilityID string            `json:"capability_id"`
	Name         string            `json:"name,omitempty"`
	Description  string            `json:"description,omitempty"`
	Source       string            `json:"source,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ReviewDecision is the result of human or automated review.
type ReviewDecision struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}

// PromotionResult is the outcome of promoting a candidate into runtime.
type PromotionResult struct {
	CandidateID    string `json:"candidate_id"`
	RegisteredName string `json:"registered_name,omitempty"`
	Outcome        string `json:"outcome,omitempty"`
}

// Observation is runtime feedback about a promoted candidate.
type Observation struct {
	CandidateID string            `json:"candidate_id"`
	Success     bool              `json:"success"`
	Reason      string            `json:"reason,omitempty"`
	Metrics     map[string]string `json:"metrics,omitempty"`
	ObservedAt  time.Time         `json:"observed_at"`
}

// GenerationStage converts a detected gap into a reviewed candidate.
type GenerationStage interface {
	GenerateCandidate(ctx context.Context, gap Gap) (*Candidate, error)
}

// ReviewStage approves or rejects a candidate before promotion.
type ReviewStage interface {
	ReviewCandidate(ctx context.Context, candidate Candidate) (ReviewDecision, error)
}

// PromotionStage promotes and rolls back candidates.
type PromotionStage interface {
	PromoteCandidate(ctx context.Context, candidateID string) (*PromotionResult, error)
	RollbackCandidate(ctx context.Context, candidateID, reason string) error
}

// GapHandler is implemented by the canonical pipeline and can be depended on
// by detectors without importing the concrete Pipeline type.
type GapHandler interface {
	HandleGap(ctx context.Context, gap Gap) (*Candidate, *PromotionResult, error)
}

// Observer receives pipeline events and promoted-candidate observations.
type Observer interface {
	ObserveSkillGrowth(ctx context.Context, event Event)
	ObservePromotion(ctx context.Context, observation Observation)
}

// PipelineConfig controls how much of the canonical pipeline is allowed to run.
type PipelineConfig struct {
	Enabled     bool `json:"enabled"`
	AutoReview  bool `json:"auto_review"`
	AutoPromote bool `json:"auto_promote"`
}

func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Enabled:     true,
		AutoReview:  false,
		AutoPromote: false,
	}
}

// Pipeline coordinates the canonical skill-growth data flow.
type Pipeline struct {
	config    PipelineConfig
	generator GenerationStage
	reviewer  ReviewStage
	promoter  PromotionStage
	observer  Observer
}

func NewPipeline(cfg PipelineConfig) *Pipeline {
	return &Pipeline{config: cfg}
}

func (p *Pipeline) SetGenerator(stage GenerationStage) { p.generator = stage }
func (p *Pipeline) SetReviewer(stage ReviewStage)      { p.reviewer = stage }
func (p *Pipeline) SetPromoter(stage PromotionStage)   { p.promoter = stage }
func (p *Pipeline) SetObserver(observer Observer)      { p.observer = observer }

// HandleGap runs a detected capability gap through the configured pipeline.
// It is safe for partially configured deployments: missing stages stop the
// flow and return the current candidate/result with a reasoned error.
func (p *Pipeline) HandleGap(ctx context.Context, gap Gap) (*Candidate, *PromotionResult, error) {
	if p == nil || !p.config.Enabled {
		return nil, nil, fmt.Errorf("skill growth pipeline disabled")
	}
	if gap.DetectedAt.IsZero() {
		gap.DetectedAt = time.Now()
	}
	p.emit(ctx, StageDetect, gap.CapabilityID, "", "detected", gap.Source)

	if p.generator == nil {
		return nil, nil, fmt.Errorf("skill growth generator not configured")
	}
	candidate, err := p.generator.GenerateCandidate(ctx, gap)
	if err != nil {
		p.emit(ctx, StageGenerate, gap.CapabilityID, "", "failed", err.Error())
		return nil, nil, err
	}
	if candidate == nil {
		err := fmt.Errorf("skill growth generator returned nil candidate")
		p.emit(ctx, StageGenerate, gap.CapabilityID, "", "failed", err.Error())
		return nil, nil, err
	}
	p.emit(ctx, StageGenerate, candidate.CapabilityID, candidate.ID, "candidate", candidate.Source)

	if !p.config.AutoReview {
		p.emit(ctx, StageReview, candidate.CapabilityID, candidate.ID, "pending", "auto review disabled")
		return candidate, nil, nil
	}

	decision := ReviewDecision{Approved: true, Reason: "auto review"}
	if p.reviewer != nil {
		decision, err = p.reviewer.ReviewCandidate(ctx, *candidate)
		if err != nil {
			p.emit(ctx, StageReview, candidate.CapabilityID, candidate.ID, "failed", err.Error())
			return candidate, nil, err
		}
	}
	if !decision.Approved {
		p.emit(ctx, StageReview, candidate.CapabilityID, candidate.ID, "rejected", decision.Reason)
		return candidate, nil, nil
	}
	p.emit(ctx, StageReview, candidate.CapabilityID, candidate.ID, "approved", decision.Reason)

	if !p.config.AutoPromote {
		p.emit(ctx, StagePromote, candidate.CapabilityID, candidate.ID, "pending", "auto promote disabled")
		return candidate, nil, nil
	}
	if p.promoter == nil {
		return candidate, nil, fmt.Errorf("skill growth promoter not configured")
	}
	result, err := p.promoter.PromoteCandidate(ctx, candidate.ID)
	if err != nil {
		p.emit(ctx, StagePromote, candidate.CapabilityID, candidate.ID, "failed", err.Error())
		return candidate, nil, err
	}
	outcome := ""
	if result != nil {
		outcome = result.Outcome
	}
	if outcome == "" {
		outcome = "promoted"
	}
	p.emit(ctx, StagePromote, candidate.CapabilityID, candidate.ID, outcome, "")
	return candidate, result, nil
}

func (p *Pipeline) emit(ctx context.Context, stage Stage, capabilityID, candidateID, outcome, reason string) {
	event := Event{
		Stage:        stage,
		CapabilityID: capabilityID,
		CandidateID:  candidateID,
		Outcome:      outcome,
		Reason:       reason,
		At:           time.Now(),
	}
	slog.Info("skillgrowth: pipeline event", "stage", stage, "capability", capabilityID, "candidate", candidateID, "outcome", outcome)
	if p.observer != nil {
		p.observer.ObserveSkillGrowth(ctx, event)
	}
}
