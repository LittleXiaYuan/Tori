package gateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/cognikernel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	"yunque-agent/pkg/packruntime"
)

// wires a gateway whose reflective loop records into a real ExperienceStore,
// using a stub evaluator that always rates the turn highly.
func newReflectionTestGateway(t *testing.T) (*Gateway, *reflectpkg.ExperienceStore) {
	t.Helper()
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "exp.json"))
	rl := cognikernel.NewReflectiveLoop()
	rl.SetReflectEval(func(ctx context.Context, intent, reply string, skills []string) (*cognikernel.ReflectEvalResult, error) {
		return &cognikernel.ReflectEvalResult{Satisfied: true, Quality: 9}, nil
	})
	rl.SetExperienceRecord(func(source, category, outcome, lesson, lctx string, tags []string) {
		store.Add(reflectpkg.Experience{Source: source, Category: category, Outcome: outcome, Lesson: lesson, Context: lctx, Tags: tags})
	})
	return &Gateway{reflectiveLoop: rl, experienceStore: store}, store
}

func waitForExperience(t *testing.T, store *reflectpkg.ExperienceStore, want int) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if n := len(store.All()); n >= want {
			return n
		}
		time.Sleep(20 * time.Millisecond)
	}
	return len(store.All())
}

// The ignition: firing reflection after a turn records an experience, and that
// experience becomes a compiled strategy — the exact string the prompt builder
// injects into the next conversation. This closes the previously-dead loop.
func TestFireReflectionRecordsExperienceAndCompilesStrategy(t *testing.T) {
	g, store := newReflectionTestGateway(t)

	g.fireReflection("tenant-1", "sess-1", "帮我把报告导出成 PDF", "已为你导出 report.pdf", []string{"file_search"}, "smart")

	if n := waitForExperience(t, store, 1); n != 1 {
		t.Fatalf("expected 1 experience recorded by ignition, got %d", n)
	}
	if strat := store.CompileStrategies(20); strat == "" {
		t.Fatal("recorded experience did not compile into any strategy (prompt injection source is empty)")
	}
}

// The pack gate: with the evolution pack disabled, firing reflection is a
// no-op — toggling the pack changes whether the agent learns.
func TestFireReflectionGatedByEvolutionPack(t *testing.T) {
	g, store := newReflectionTestGateway(t)

	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Install(packruntime.Manifest{
		ID: evolutionPackID, Name: "Inner Life", Version: "0.1.0", Optional: true, DefaultState: "disabled",
	}, "test"); err != nil {
		t.Fatal(err)
	}
	g.packRegistry = registry

	if g.evolutionEnabled() {
		t.Fatal("evolution should be disabled when inner-life pack is disabled")
	}
	g.fireReflection("tenant-1", "sess-1", "intent", "reply", nil, "smart")
	if n := waitForExperience(t, store, 1); n != 0 {
		t.Fatalf("disabled evolution pack must block learning, got %d experiences", n)
	}

	// Enabling the pack re-opens the loop.
	if _, err := registry.Enable(evolutionPackID); err != nil {
		t.Fatal(err)
	}
	if !g.evolutionEnabled() {
		t.Fatal("evolution should be enabled after enabling the pack")
	}
	g.fireReflection("tenant-1", "sess-1", "intent", "reply", nil, "smart")
	if n := waitForExperience(t, store, 1); n != 1 {
		t.Fatalf("enabled pack should allow learning, got %d", n)
	}
}

// The 小羽/API mode gate: API模式 sessions are explicit pass-throughs to an
// external model and must not feed self-distill sample collection — only
// 小羽模式 (the default, unset mode) keeps learning.
func TestFireReflectionGatedByAPIMode(t *testing.T) {
	g, store := newReflectionTestGateway(t)
	g.providerReg = llm.NewProviderRegistry(nil)

	g.providerReg.SetSessionMode("api-sess", "api")
	g.fireReflection("tenant-1", "api-sess", "intent", "reply", nil, "smart")
	if n := waitForExperience(t, store, 1); n != 0 {
		t.Fatalf("API模式 session must not record experience, got %d", n)
	}

	g.fireReflection("tenant-1", "xiaoyu-sess", "intent", "reply", nil, "smart")
	if n := waitForExperience(t, store, 1); n != 1 {
		t.Fatalf("小羽模式 (unset mode) session should still learn, got %d", n)
	}
}
