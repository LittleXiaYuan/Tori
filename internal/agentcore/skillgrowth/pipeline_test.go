package skillgrowth

import (
	"context"
	"testing"
)

type fakeGenerator struct {
	candidate *Candidate
	err       error
}

func (g fakeGenerator) GenerateCandidate(ctx context.Context, gap Gap) (*Candidate, error) {
	if g.err != nil {
		return nil, g.err
	}
	c := *g.candidate
	if c.CapabilityID == "" {
		c.CapabilityID = gap.CapabilityID
	}
	return &c, nil
}

type fakePromoter struct {
	promoted string
}

func (p *fakePromoter) PromoteCandidate(ctx context.Context, candidateID string) (*PromotionResult, error) {
	p.promoted = candidateID
	return &PromotionResult{CandidateID: candidateID, RegisteredName: "registered_skill", Outcome: "promoted"}, nil
}

func (p *fakePromoter) RollbackCandidate(ctx context.Context, candidateID, reason string) error {
	return nil
}

type eventRecorder struct {
	events []Event
}

func (r *eventRecorder) ObserveSkillGrowth(ctx context.Context, event Event) {
	r.events = append(r.events, event)
}

func (r *eventRecorder) ObservePromotion(ctx context.Context, observation Observation) {}

func TestPipelineStopsAtReviewWhenAutoReviewDisabled(t *testing.T) {
	rec := &eventRecorder{}
	p := NewPipeline(DefaultPipelineConfig())
	p.SetObserver(rec)
	p.SetGenerator(fakeGenerator{candidate: &Candidate{ID: "candidate-1", Source: "test"}})

	candidate, result, err := p.HandleGap(context.Background(), Gap{CapabilityID: "cap.test", Source: "unit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if candidate == nil || candidate.ID != "candidate-1" {
		t.Fatalf("unexpected candidate: %#v", candidate)
	}
	if result != nil {
		t.Fatalf("expected no promotion result, got %#v", result)
	}
	if len(rec.events) != 3 {
		t.Fatalf("events len = %d, want 3: %#v", len(rec.events), rec.events)
	}
	if rec.events[2].Stage != StageReview || rec.events[2].Outcome != "pending" {
		t.Fatalf("unexpected review event: %#v", rec.events[2])
	}
}

func TestPipelineCanAutoPromote(t *testing.T) {
	promoter := &fakePromoter{}
	p := NewPipeline(PipelineConfig{Enabled: true, AutoReview: true, AutoPromote: true})
	p.SetGenerator(fakeGenerator{candidate: &Candidate{ID: "candidate-2", Source: "test"}})
	p.SetPromoter(promoter)

	candidate, result, err := p.HandleGap(context.Background(), Gap{CapabilityID: "cap.promote", Source: "unit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if candidate == nil || candidate.ID != "candidate-2" {
		t.Fatalf("unexpected candidate: %#v", candidate)
	}
	if result == nil || result.RegisteredName != "registered_skill" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if promoter.promoted != "candidate-2" {
		t.Fatalf("promoted candidate = %q, want candidate-2", promoter.promoted)
	}
}
