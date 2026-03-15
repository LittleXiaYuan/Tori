package router

import (
	"context"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/models"
)

func setupRegistry() *models.Registry {
	reg := models.NewRegistry()
	reg.Register(models.Model{ID: "fast", ModelID: "gpt-4o-mini", Name: "Fast", Type: models.TypeChat, ClientType: models.ClientOpenAI})
	reg.Register(models.Model{ID: "smart", ModelID: "gpt-4o", Name: "Smart", Type: models.TypeChat, ClientType: models.ClientOpenAI})
	reg.Register(models.Model{ID: "expert", ModelID: "o1", Name: "Expert", Type: models.TypeChat, ClientType: models.ClientOpenAI, SupportsReasoning: true})
	return reg
}

func TestClassifySimpleGreeting(t *testing.T) {
	reg := setupRegistry()
	r := New(reg)
	r.SetSlot(TierFast, "fast")
	r.SetSlot(TierSmart, "smart")
	r.SetSlot(TierExpert, "expert")

	m, tier := r.Route(context.Background(), "你好", false)
	if tier != TierFast {
		t.Fatalf("expected TierFast for greeting, got %s", tier)
	}
	if m.ModelID != "gpt-4o-mini" {
		t.Fatalf("expected gpt-4o-mini, got %s", m.ModelID)
	}
}

func TestClassifyCodeQuery(t *testing.T) {
	reg := setupRegistry()
	r := New(reg)
	r.SetSlot(TierFast, "fast")
	r.SetSlot(TierSmart, "smart")
	r.SetSlot(TierExpert, "expert")

	m, tier := r.Route(context.Background(), "请帮我实现一个二叉树的遍历算法，并且分析时间复杂度", false)
	if tier != TierExpert {
		t.Fatalf("expected TierExpert for code query, got %s", tier)
	}
	if m.ModelID != "o1" {
		t.Fatalf("expected o1, got %s", m.ModelID)
	}
}

func TestClassifyMediumQuery(t *testing.T) {
	reg := setupRegistry()
	r := New(reg)
	r.SetSlot(TierFast, "fast")
	r.SetSlot(TierSmart, "smart")
	r.SetSlot(TierExpert, "expert")

	_, tier := r.Route(context.Background(), "帮我整理一下这个会议的要点", false)
	if tier == TierFast {
		t.Fatalf("meeting summary should not be TierFast, got %s", tier)
	}
}

func TestImageForcesExpert(t *testing.T) {
	reg := setupRegistry()
	r := New(reg)
	r.SetSlot(TierFast, "fast")
	r.SetSlot(TierSmart, "smart")
	r.SetSlot(TierExpert, "expert")

	_, tier := r.Route(context.Background(), "这张图片是什么", true)
	if tier != TierExpert {
		t.Fatalf("image query should be TierExpert, got %s", tier)
	}
}

func TestFallbackToPrimary(t *testing.T) {
	reg := setupRegistry()
	reg.SetPrimary("smart")
	r := New(reg)
	// No slots configured

	m, _ := r.Route(context.Background(), "hello", false)
	if m == nil {
		t.Fatal("expected fallback to primary model")
	}
	if m.ModelID != "gpt-4o" {
		t.Fatalf("expected gpt-4o as fallback, got %s", m.ModelID)
	}
}

func TestRecordLatency(t *testing.T) {
	reg := setupRegistry()
	r := New(reg)

	r.RecordLatency("gpt-4o-mini", 100*time.Millisecond)
	r.RecordLatency("gpt-4o-mini", 200*time.Millisecond)

	stats := r.GetStats()
	latency := stats["latency"].(map[string]string)
	if _, ok := latency["gpt-4o-mini"]; !ok {
		t.Fatal("expected latency record for gpt-4o-mini")
	}
}

func TestGetSlots(t *testing.T) {
	reg := setupRegistry()
	r := New(reg)
	r.SetSlot(TierFast, "fast")
	r.SetSlot(TierExpert, "expert")

	slots := r.GetSlots()
	if slots["fast"] != "fast" {
		t.Fatalf("expected slot fast=fast, got %s", slots["fast"])
	}
	if slots["expert"] != "expert" {
		t.Fatalf("expected slot expert=expert, got %s", slots["expert"])
	}
}
