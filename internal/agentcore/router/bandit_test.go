package router

import (
	"testing"
)

func TestBanditRegisterAndSelect(t *testing.T) {
	b := NewModelBandit(PolicyUCB1)
	b.RegisterArm(TierFast, "gpt-4o-mini")
	b.RegisterArm(TierFast, "deepseek-chat")

	model, ok := b.Select(TierFast)
	if !ok {
		t.Fatal("expected selection from registered arms")
	}
	if model != "gpt-4o-mini" && model != "deepseek-chat" {
		t.Errorf("unexpected model: %s", model)
	}
}

func TestBanditNoArms(t *testing.T) {
	b := NewModelBandit(PolicyUCB1)
	_, ok := b.Select(TierExpert)
	if ok {
		t.Error("expected no selection when no arms registered")
	}
}

func TestBanditUCB1Convergence(t *testing.T) {
	b := NewModelBandit(PolicyUCB1)
	b.RegisterArm(TierSmart, "good-model")
	b.RegisterArm(TierSmart, "bad-model")

	for i := 0; i < 100; i++ {
		b.RecordOutcome(TierSmart, "good-model", 0.9, 200)
		b.RecordOutcome(TierSmart, "bad-model", 0.2, 500)
	}

	counts := map[string]int{}
	for i := 0; i < 50; i++ {
		model, _ := b.Select(TierSmart)
		counts[model]++
	}

	if counts["good-model"] < counts["bad-model"] {
		t.Errorf("UCB1 should prefer good-model after 100 observations, got good=%d bad=%d",
			counts["good-model"], counts["bad-model"])
	}
}

func TestBanditThompsonConvergence(t *testing.T) {
	b := NewModelBandit(PolicyThompson)
	b.RegisterArm(TierSmart, "good-model")
	b.RegisterArm(TierSmart, "bad-model")

	for i := 0; i < 100; i++ {
		b.RecordOutcome(TierSmart, "good-model", 0.9, 200)
		b.RecordOutcome(TierSmart, "bad-model", 0.2, 500)
	}

	counts := map[string]int{}
	for i := 0; i < 50; i++ {
		model, _ := b.Select(TierSmart)
		counts[model]++
	}

	if counts["good-model"] < counts["bad-model"] {
		t.Errorf("Thompson should prefer good-model after 100 observations, got good=%d bad=%d",
			counts["good-model"], counts["bad-model"])
	}
}

func TestBanditExploration(t *testing.T) {
	b := NewModelBandit(PolicyUCB1)
	b.RegisterArm(TierFast, "model-a")
	b.RegisterArm(TierFast, "model-b")
	b.RegisterArm(TierFast, "model-c")

	// With no observations, UCB1 should explore all arms
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		model, _ := b.Select(TierFast)
		seen[model] = true
		b.RecordOutcome(TierFast, model, 0.5, 300)
	}
	if len(seen) < 2 {
		t.Errorf("UCB1 should explore at least 2 arms in first 10 pulls, saw %d", len(seen))
	}
}

func TestBanditSnapshot(t *testing.T) {
	b := NewModelBandit(PolicyUCB1)
	b.RegisterArm(TierFast, "model-a")
	b.RecordOutcome(TierFast, "model-a", 0.8, 150)
	b.RecordOutcome(TierFast, "model-a", 0.6, 200)

	snap := b.Snapshot()
	if len(snap.Arms) != 1 {
		t.Fatalf("expected 1 arm in snapshot, got %d", len(snap.Arms))
	}
	arm := snap.Arms[0]
	if arm.Pulls != 2 {
		t.Errorf("expected 2 pulls, got %d", arm.Pulls)
	}
	if arm.Successes != 2 {
		t.Errorf("expected 2 successes (0.8 and 0.6 >= 0.5), got %d", arm.Successes)
	}
	if arm.AvgReward < 0.6 || arm.AvgReward > 0.8 {
		t.Errorf("expected avg reward ~0.7, got %.3f", arm.AvgReward)
	}
}

func TestBanditSingleArm(t *testing.T) {
	b := NewModelBandit(PolicyThompson)
	b.RegisterArm(TierExpert, "only-model")

	model, ok := b.Select(TierExpert)
	if !ok || model != "only-model" {
		t.Errorf("single arm should always be selected")
	}
}

func TestBanditMultiTier(t *testing.T) {
	b := NewModelBandit(PolicyUCB1)
	b.RegisterArm(TierFast, "mini-model")
	b.RegisterArm(TierSmart, "mid-model")
	b.RegisterArm(TierExpert, "big-model")

	fast, _ := b.Select(TierFast)
	smart, _ := b.Select(TierSmart)
	expert, _ := b.Select(TierExpert)

	if fast != "mini-model" || smart != "mid-model" || expert != "big-model" {
		t.Errorf("each tier should return its own model: fast=%s smart=%s expert=%s",
			fast, smart, expert)
	}
}
