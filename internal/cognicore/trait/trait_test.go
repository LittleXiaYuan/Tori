package trait

import (
	"context"
	"os"
	"testing"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tori-trait-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestStoreAdd(t *testing.T) {
	s := NewStore(tmpDir(t))
	tr := s.Add(DimCommunicationStyle, "concise", 0.8, "user said be brief")
	if tr.ID == "" {
		t.Fatal("expected ID")
	}
	if tr.HitCount != 1 {
		t.Fatal("expected hit 1")
	}
}

func TestStoreReinforce(t *testing.T) {
	s := NewStore(tmpDir(t))
	s.Add(DimCommunicationStyle, "concise", 0.5, "msg1")
	s.Add(DimCommunicationStyle, "concise", 0.6, "msg2") // reinforce

	tr, ok := s.Get(DimCommunicationStyle, "concise")
	if !ok {
		t.Fatal("not found")
	}
	if tr.HitCount != 2 {
		t.Fatalf("expected 2 hits, got %d", tr.HitCount)
	}
	if tr.Confidence <= 0.5 {
		t.Fatal("confidence should increase on reinforce")
	}
}

func TestStoreByDimension(t *testing.T) {
	s := NewStore(tmpDir(t))
	s.Add(DimDomainPreference, "Go", 0.9, "")
	s.Add(DimDomainPreference, "AI", 0.7, "")
	s.Add(DimTonePreference, "casual", 0.6, "")

	domain := s.ByDimension(DimDomainPreference)
	if len(domain) != 2 {
		t.Fatalf("expected 2, got %d", len(domain))
	}
}

func TestStoreTopTraits(t *testing.T) {
	s := NewStore(tmpDir(t))
	s.Add("a", "low", 0.2, "")
	s.Add("b", "mid", 0.5, "")
	s.Add("c", "high", 0.9, "")

	top := s.TopTraits(2)
	if len(top) != 2 {
		t.Fatalf("expected 2, got %d", len(top))
	}
	if top[0].Preference != "high" {
		t.Fatal("expected highest first")
	}
}

func TestStoreRemove(t *testing.T) {
	s := NewStore(tmpDir(t))
	s.Add("d", "p", 0.5, "")
	s.Remove("d", "p")
	if _, ok := s.Get("d", "p"); ok {
		t.Fatal("should be removed")
	}
}

func TestStorePersistAndLoad(t *testing.T) {
	dir := tmpDir(t)
	s := NewStore(dir)
	s.Add(DimExpertiseLevel, "advanced", 0.85, "")

	s2 := NewStore(dir)
	all := s2.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 persisted, got %d", len(all))
	}
	if all[0].Preference != "advanced" {
		t.Fatal("wrong preference")
	}
}

func TestForPersonaPrompt(t *testing.T) {
	s := NewStore(tmpDir(t))
	s.Add(DimCommunicationStyle, "concise", 0.9, "")
	s.Add(DimDomainPreference, "Go", 0.8, "")

	prompt := s.ForPersonaPrompt(5)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !contains(prompt, "concise") || !contains(prompt, "Go") {
		t.Fatal("missing traits in prompt")
	}
}

func TestForPersonaPromptEmpty(t *testing.T) {
	s := NewStore(tmpDir(t))
	if s.ForPersonaPrompt(5) != "" {
		t.Fatal("expected empty")
	}
}

func TestMiner(t *testing.T) {
	s := NewStore(tmpDir(t))
	miner := NewMiner(s, func(ctx context.Context, msg string) ([]MineResult, error) {
		return []MineResult{
			{Dimension: DimCommunicationStyle, Preference: "technical", Confidence: 0.8},
			{Dimension: DimContentInterest, Preference: "AI agents", Confidence: 0.2}, // below threshold
		}, nil
	})

	stored, err := miner.Mine(context.Background(), "I prefer technical explanations")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored (filtered low conf), got %d", len(stored))
	}
	if stored[0].Dimension != DimCommunicationStyle {
		t.Fatal("wrong dimension")
	}
}

func TestMinerNoFunc(t *testing.T) {
	s := NewStore(tmpDir(t))
	miner := NewMiner(s, nil)
	_, err := miner.Mine(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMinerCustomThreshold(t *testing.T) {
	s := NewStore(tmpDir(t))
	miner := NewMiner(s, func(ctx context.Context, msg string) ([]MineResult, error) {
		return []MineResult{
			{Dimension: "d", Preference: "p", Confidence: 0.5},
		}, nil
	})
	miner.SetMinConfidence(0.6)

	stored, _ := miner.Mine(context.Background(), "test")
	if len(stored) != 0 {
		t.Fatal("should be filtered by threshold")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
