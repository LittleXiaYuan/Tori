package gateway

import (
	"errors"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/models"
	"yunque-agent/internal/agentcore/router"
)

func TestRecordRouterOutcomeUpdatesBanditAndLatency(t *testing.T) {
	gw, _ := newTestGateway()
	reg := models.NewRegistry()
	reg.Register(models.Model{ModelID: "smart-model", Type: models.TypeChat, ClientType: models.ClientOpenAI})
	r := router.New(reg)
	b := router.NewModelBandit(router.PolicyUCB1)
	b.RegisterArm(router.TierSmart, "smart-model")
	r.SetBandit(b)
	gw.SetSmartRouter(r)

	gw.recordRouterOutcome("smart", "smart-model", time.Now().Add(-25*time.Millisecond), nil, "这是一次足够完整的 planner 回复，用来证明成功结果会反馈给 bandit。", nil)

	snap := b.Snapshot()
	if len(snap.Arms) != 1 {
		t.Fatalf("expected one bandit arm, got %d", len(snap.Arms))
	}
	arm := snap.Arms[0]
	if arm.Pulls != 1 {
		t.Fatalf("expected one pull, got %d", arm.Pulls)
	}
	if arm.Successes != 1 || arm.Failures != 0 {
		t.Fatalf("expected successful outcome, got successes=%d failures=%d", arm.Successes, arm.Failures)
	}
	if arm.AvgReward <= 0.5 {
		t.Fatalf("expected positive reward, got %.3f", arm.AvgReward)
	}

	stats := r.GetStats()
	latency := stats["latency"].(map[string]string)
	if latency["smart-model"] == "" {
		t.Fatalf("expected router latency to be recorded, got %#v", latency)
	}
}

func TestRecordRouterOutcomeRecordsPlannerFailure(t *testing.T) {
	gw, _ := newTestGateway()
	reg := models.NewRegistry()
	reg.Register(models.Model{ModelID: "fast-model", Type: models.TypeChat, ClientType: models.ClientOpenAI})
	r := router.New(reg)
	b := router.NewModelBandit(router.PolicyUCB1)
	b.RegisterArm(router.TierFast, "fast-model")
	r.SetBandit(b)
	gw.SetSmartRouter(r)

	gw.recordRouterOutcome("fast", "fast-model", time.Now().Add(-10*time.Millisecond), errors.New("planner timeout"), "", nil)

	arm := b.Snapshot().Arms[0]
	if arm.Pulls != 1 {
		t.Fatalf("expected one failed pull, got %d", arm.Pulls)
	}
	if arm.Successes != 0 || arm.Failures != 1 {
		t.Fatalf("expected failed outcome, got successes=%d failures=%d", arm.Successes, arm.Failures)
	}
	if arm.AvgReward != 0 {
		t.Fatalf("expected zero reward for planner error, got %.3f", arm.AvgReward)
	}
}
