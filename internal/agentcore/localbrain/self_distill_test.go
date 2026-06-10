package localbrain

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestConversationScorer_Heuristic(t *testing.T) {
	scorer := NewConversationScorer(nil)

	tests := []struct {
		name     string
		sample   DistillSample
		minScore float64
		maxScore float64
	}{
		{
			name: "satisfied detailed reply",
			sample: DistillSample{
				Input:     "如何配置定时任务？",
				Output:    "您可以通过以下步骤配置定时任务：1. 进入设置页面 2. 选择「定时任务」3. 点击新建 4. 设置触发条件和执行动作 5. 保存并启用。如果需要使用 cron 表达式，可以参考文档。",
				Satisfied: true,
				Tier:      "smart",
			},
			minScore: 0.7,
			maxScore: 1.0,
		},
		{
			name: "unsatisfied terse reply",
			sample: DistillSample{
				Input:     "这个功能怎么用？",
				Output:    "不支持。",
				Satisfied: false,
			},
			minScore: 0.0,
			maxScore: 0.5,
		},
		{
			name: "upgraded task",
			sample: DistillSample{
				Input:     "分析这份报告",
				Output:    "根据分析，报告显示了三个关键趋势...",
				Satisfied: true,
				Upgraded:  true,
				Tier:      "expert",
			},
			minScore: 0.5,
			maxScore: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scored := scorer.ScoreHeuristic(tt.sample)
			if scored.Score < tt.minScore || scored.Score > tt.maxScore {
				t.Errorf("score %.2f not in [%.2f, %.2f]", scored.Score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestConversationScorer_ScoreBatch(t *testing.T) {
	scorer := NewConversationScorer(nil)

	samples := []DistillSample{
		{Input: "hello", Output: "Hello! How can I help you?", Satisfied: true},
		{Input: "test", Output: "ok", Satisfied: false},
	}

	scored := scorer.ScoreBatch(context.Background(), samples)
	if len(scored) != 2 {
		t.Fatalf("expected 2 scored samples, got %d", len(scored))
	}
	if scored[0].Score <= scored[1].Score {
		t.Errorf("expected first sample (satisfied) to score higher than second (unsatisfied)")
	}
}

func TestDefaultSelfDistillConfig(t *testing.T) {
	cfg := DefaultSelfDistillConfig()
	if cfg.BaseModel == "" {
		t.Error("expected non-empty base model")
	}
	if cfg.MinSamples <= 0 {
		t.Error("expected positive min samples")
	}
	if cfg.LoRARank <= 0 {
		t.Error("expected positive lora rank")
	}
}

func TestSelfDistillPipeline_InsufficientData(t *testing.T) {
	pipeline := NewSelfDistillPipeline(nil, nil, nil, nil, nil, nil, t.TempDir())

	cfg := DefaultSelfDistillConfig()
	cfg.MinSamples = 100

	report := pipeline.Run(context.Background(), cfg)
	if report.Success {
		t.Error("expected failure due to insufficient data")
	}
	if report.Error == "" {
		t.Error("expected error message")
	}
}

func TestStepExport_BuildsPersonaMemorySystem(t *testing.T) {
	p := NewSelfDistillPipeline(nil, nil, nil, nil, nil, nil, t.TempDir())
	cfg := DefaultSelfDistillConfig()
	cfg.MinSamples = 1
	cfg.MinScore = 0.0
	cfg.DefaultPersona = "你是小羽。"

	scored := []ScoredSample{
		// has its own persona + recalled memory
		{DistillSample: DistillSample{
			Input: "在吗", Output: "在的~",
			Persona: "你是小羽。温柔体贴。",
			Memory:  "<recalled_memories>用户喜欢简短回复</recalled_memories>",
		}, Score: 1.0},
		// no per-sample persona → uses cfg.DefaultPersona
		{DistillSample: DistillSample{Input: "几点了", Output: "三点"}, Score: 1.0},
	}

	report := &DistillReport{}
	path := p.stepExport(scored, cfg, report)
	if path == "" {
		t.Fatalf("export failed: %s", report.Error)
	}
	data, _ := os.ReadFile(path)
	text := string(data)

	if strings.Contains(text, "You are a helpful assistant.") {
		t.Fatalf("persona present but export still emits generic assistant system:\n%s", text)
	}
	if !strings.Contains(text, "你是小羽。温柔体贴。") {
		t.Fatalf("per-sample persona missing from system:\n%s", text)
	}
	if !strings.Contains(text, "recalled_memories") {
		t.Fatalf("recalled memory block missing from system:\n%s", text)
	}
	if !strings.Contains(text, "你是小羽。") {
		t.Fatalf("default persona missing for persona-less sample:\n%s", text)
	}
}

func TestStepExport_NeutralFallbackWhenNoPersona(t *testing.T) {
	p := NewSelfDistillPipeline(nil, nil, nil, nil, nil, nil, t.TempDir())
	cfg := DefaultSelfDistillConfig()
	cfg.MinSamples = 1
	cfg.MinScore = 0.0
	// no DefaultPersona, sample has no persona → neutral fallback (generic base)
	scored := []ScoredSample{{DistillSample: DistillSample{Input: "hi", Output: "hello"}, Score: 1.0}}

	report := &DistillReport{}
	path := p.stepExport(scored, cfg, report)
	if path == "" {
		t.Fatalf("export failed: %s", report.Error)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "You are a helpful assistant.") {
		t.Fatalf("expected neutral fallback when no persona/default:\n%s", string(data))
	}
}

func TestScoreDistribution(t *testing.T) {
	scorer := NewConversationScorer(nil)

	samples := []DistillSample{
		{Input: "q1", Output: "Very long detailed answer with lots of helpful information and examples that should score well because it demonstrates expertise and thoroughness", Satisfied: true, Tier: "expert"},
		{Input: "q2", Output: "Short.", Satisfied: false},
	}

	scored := scorer.ScoreBatch(context.Background(), samples)
	if len(scored) < 2 {
		t.Fatal("expected at least 2 scored samples")
	}
}
