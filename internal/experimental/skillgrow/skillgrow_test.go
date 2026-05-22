package skillgrow

import (
	"context"
	"testing"

	"yunque-agent/internal/agentcore/skillgrowth"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector(0) // below minimum → should default to 3
	if d.threshold < 2 {
		t.Errorf("threshold = %d, want >= 2", d.threshold)
	}
}

func TestDetectorObserveNoMemSearch(t *testing.T) {
	d := NewDetector(3)
	// Should not panic even without memSearch
	d.Observe(context.Background(), "test query")
}

func TestDetectorObserveBelowThreshold(t *testing.T) {
	d := NewDetector(3)
	proposalCalled := false
	d.SetOnProposal(func(ctx context.Context, pattern, suggestion string) {
		proposalCalled = true
	})
	d.SetMemSearch(func(ctx context.Context, query string) (int, string) {
		return 1, "sample" // below threshold
	})

	d.Observe(context.Background(), "hello")
	if proposalCalled {
		t.Error("proposal should not be called below threshold")
	}
}

func TestDetectorObserveAboveThreshold(t *testing.T) {
	d := NewDetector(3)
	proposalCalled := false
	var gotGap skillgrowth.Gap
	d.SetOnProposal(func(ctx context.Context, pattern, suggestion string) {
		proposalCalled = true
	})
	d.SetOnGap(func(ctx context.Context, gap skillgrowth.Gap) {
		gotGap = gap
	})
	d.SetMemSearch(func(ctx context.Context, query string) (int, string) {
		return 5, "sample response" // above threshold
	})

	d.Observe(context.Background(), "deploy to staging")
	if !proposalCalled {
		t.Error("expected proposal callback")
	}
	if gotGap.CapabilityID == "" {
		t.Fatal("expected canonical gap callback")
	}
	if gotGap.Source != "skillgrow.detector" {
		t.Fatalf("unexpected gap source: %q", gotGap.Source)
	}
	if gotGap.Evidence["count"] != "5" {
		t.Fatalf("unexpected gap evidence: %#v", gotGap.Evidence)
	}
}

func TestDetectorPatterns(t *testing.T) {
	d := NewDetector(2)
	d.SetMemSearch(func(ctx context.Context, query string) (int, string) {
		return 3, "sample"
	})
	d.SetOnProposal(func(ctx context.Context, pattern, suggestion string) {})

	d.Observe(context.Background(), "deploy app")
	patterns := d.Patterns()
	if len(patterns) != 1 {
		t.Errorf("patterns len = %d, want 1", len(patterns))
	}
}

func TestDetectorReset(t *testing.T) {
	d := NewDetector(2)
	d.SetMemSearch(func(ctx context.Context, query string) (int, string) { return 5, "s" })
	d.SetOnProposal(func(ctx context.Context, p, s string) {})
	d.Observe(context.Background(), "test")

	d.Reset()
	if len(d.Patterns()) != 0 {
		t.Error("expected empty patterns after Reset")
	}
}

// ── Generator tests ──

func TestNewGenerator(t *testing.T) {
	g := NewGenerator(nil)
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
	if len(g.Proposals()) != 0 {
		t.Error("expected empty proposals")
	}
}

func TestGenerateFromPatternNoLLM(t *testing.T) {
	g := NewGenerator(nil)
	_, err := g.GenerateFromPattern(context.Background(), Pattern{Query: "test", Count: 5})
	if err == nil {
		t.Error("expected error with no LLM")
	}
}

func TestGenerateFromPatternWithLLM(t *testing.T) {
	g := NewGenerator(func(ctx context.Context, system, user string) (string, error) {
		return `{"slug":"auto_deploy","name":"Auto Deploy","description":"Automates deployment","parameters":[{"name":"env","type":"string","description":"target environment","required":true}],"code":"func(){}","trigger":"deploy *"}`, nil
	})

	template, err := g.GenerateFromPattern(context.Background(), Pattern{Query: "deploy to staging", Count: 5, Sample: "deploy done"})
	if err != nil {
		t.Fatalf("GenerateFromPattern: %v", err)
	}
	if template.Slug != "auto_deploy" {
		t.Errorf("slug = %s, want auto_deploy", template.Slug)
	}
	if template.Source != "skillgrow" {
		t.Errorf("source = %s, want skillgrow", template.Source)
	}
	if len(template.Parameters) != 1 {
		t.Errorf("parameters len = %d, want 1", len(template.Parameters))
	}

	proposals := g.Proposals()
	if len(proposals) != 1 {
		t.Errorf("proposals len = %d, want 1", len(proposals))
	}
}

func TestFeedbackAccept(t *testing.T) {
	g := NewGenerator(func(ctx context.Context, system, user string) (string, error) {
		return `{"slug":"test_skill","name":"Test","description":"test","parameters":[],"code":"","trigger":"test"}`, nil
	})

	g.GenerateFromPattern(context.Background(), Pattern{Query: "test", Count: 3})

	err := g.Feedback("test_skill", FeedbackAccept, "")
	if err != nil {
		t.Fatalf("Feedback: %v", err)
	}

	if len(g.Proposals()) != 0 {
		t.Error("accepted proposal should be removed from proposals")
	}
	if len(g.AcceptedSkills()) != 1 {
		t.Error("accepted skill should be in accepted list")
	}
}

func TestFeedbackReject(t *testing.T) {
	g := NewGenerator(func(ctx context.Context, system, user string) (string, error) {
		return `{"slug":"bad_skill","name":"Bad","description":"bad","parameters":[],"code":"","trigger":"bad"}`, nil
	})

	g.GenerateFromPattern(context.Background(), Pattern{Query: "bad", Count: 3})
	g.Feedback("bad_skill", FeedbackReject, "")

	if len(g.Proposals()) != 0 {
		t.Error("rejected proposal should be removed")
	}
	if len(g.AcceptedSkills()) != 0 {
		t.Error("rejected skill should not be in accepted")
	}
}

func TestFeedbackUnknownSlug(t *testing.T) {
	g := NewGenerator(nil)
	err := g.Feedback("nonexistent", FeedbackAccept, "")
	if err == nil {
		t.Error("expected error for unknown slug")
	}
}

func TestGeneratorOnGeneratedCallback(t *testing.T) {
	called := false
	g := NewGenerator(func(ctx context.Context, system, user string) (string, error) {
		return `{"slug":"cb_skill","name":"CB","description":"cb","parameters":[],"code":"","trigger":"cb"}`, nil
	})
	g.SetOnGenerated(func(template SkillTemplate) {
		called = true
	})

	g.GenerateFromPattern(context.Background(), Pattern{Query: "cb", Count: 3})
	if !called {
		t.Error("expected onGenerated callback")
	}
}

func TestPipelineGeneratorAdaptsTemplate(t *testing.T) {
	g := NewGenerator(func(ctx context.Context, system, user string) (string, error) {
		return `{"slug":"pipeline_skill","name":"Pipeline Skill","description":"Generated through pipeline","parameters":[],"code":"","trigger":"pipeline"}`, nil
	})
	adapter := NewPipelineGenerator(g)

	candidate, err := adapter.GenerateCandidate(context.Background(), skillgrowth.Gap{
		CapabilityID:   "cap.pipeline",
		Description:    "repeat pipeline task",
		FailureContext: "sample",
		Evidence:       map[string]string{"count": "4"},
	})
	if err != nil {
		t.Fatalf("GenerateCandidate: %v", err)
	}
	if candidate.ID != "pipeline_skill" {
		t.Fatalf("candidate id = %q, want pipeline_skill", candidate.ID)
	}
	if candidate.CapabilityID != "cap.pipeline" {
		t.Fatalf("candidate capability = %q, want cap.pipeline", candidate.CapabilityID)
	}
	if candidate.Source != "skillgrow" {
		t.Fatalf("candidate source = %q, want skillgrow", candidate.Source)
	}
}
