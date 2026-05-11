package localbrain

import (
	"context"
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
