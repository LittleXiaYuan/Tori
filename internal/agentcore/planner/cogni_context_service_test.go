package planner

import (
	"context"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestCogniContextServiceDefaultsToNoop(t *testing.T) {
	svc := NewCogniContextService()

	if got := svc.Context(context.Background(), "msg", "tenant", "web"); got != "" {
		t.Fatalf("default context = %q, want empty", got)
	}
	in := []skills.Skill{dummyPlannerSkill("a")}
	out := svc.FilterSkills("msg", "tenant", "web", in)
	if len(out) != 1 || out[0].Name() != "a" {
		t.Fatalf("default filter changed skills: %#v", out)
	}
	if _, ok := svc.Trace("msg", "tenant", "web"); ok {
		t.Fatal("default trace should be absent")
	}
}

func TestPlannerSetCogniCallbacksUsesService(t *testing.T) {
	p := &Planner{}
	p.SetCogniContext(func(ctx context.Context, message, tenantID, channel string) string {
		return "cogni:" + message
	})
	p.SetCogniSkillFilter(func(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
		return in[:1]
	})
	p.SetCogniTrace(func(message, tenantID, channel string) (CogniTraceDetail, bool) {
		return CogniTraceDetail{Activated: []string{"demo"}}, true
	})

	if p.cogniService == nil {
		t.Fatal("expected cogni service to be initialized")
	}
	if got := p.cogniService.Context(context.Background(), "hello", "tenant", "web"); got != "cogni:hello" {
		t.Fatalf("context = %q, want cogni:hello", got)
	}
	filtered := p.cogniService.FilterSkills("hello", "tenant", "web", []skills.Skill{dummyPlannerSkill("a"), dummyPlannerSkill("b")})
	if len(filtered) != 1 || filtered[0].Name() != "a" {
		t.Fatalf("unexpected filtered skills: %#v", filtered)
	}
	trace, ok := p.cogniService.Trace("hello", "tenant", "web")
	if !ok || len(trace.Activated) != 1 || trace.Activated[0] != "demo" {
		t.Fatalf("unexpected trace: %#v ok=%v", trace, ok)
	}
}
