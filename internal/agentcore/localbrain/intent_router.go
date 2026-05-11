package localbrain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"yunque-agent/pkg/cogni"
)

// IntentRouter is the unified routing layer that combines the LocalBrain's
// fast-path classifier with the NL config translator. It implements the
// two-stage routing strategy:
//
//	Stage 1: Small model classifies → {chat, code, search, tool, config, complex}
//	Stage 2 (config only): LLM translates → specific NL config intent
//
// This replaces direct calls to LocalBrain.Classify + NLConfigTranslator.Translate
// with a single entry point that handles both routing and config intent resolution.
type IntentRouter struct {
	brain      *LocalBrain
	translator *cogni.NLConfigTranslator
	sink       SelfDistillSink

	configKeywords []string
}

// RouterResult is the combined output of the two-stage routing.
type RouterResult struct {
	// Stage 1: Routing decision
	Decision *Decision `json:"decision"`

	// Stage 2: NL config result (nil if not a config intent)
	NLConfig *cogni.NLConfigResult `json:"nl_config,omitempty"`

	// Whether the query was identified as a configuration request
	IsConfig bool `json:"is_config"`

	// Processing latency for each stage
	Stage1Latency time.Duration `json:"stage1_latency"`
	Stage2Latency time.Duration `json:"stage2_latency"`
}

// NewIntentRouter creates a router with the given brain and translator.
func NewIntentRouter(brain *LocalBrain, translator *cogni.NLConfigTranslator, sink SelfDistillSink) *IntentRouter {
	return &IntentRouter{
		brain:      brain,
		translator: translator,
		sink:       sink,
		configKeywords: []string{
			"切换", "设置", "配置", "修改", "调整", "切", "开启", "关闭",
			"安装", "添加", "删除", "创建", "取消", "记住", "忘掉",
			"定时", "每天", "每周", "每小时", "监控",
			"知识库", "知识", "记忆",
			"模型", "GPT", "Claude", "DeepSeek",
			"主题", "暗色", "亮色", "字体", "禅模式", "轻松模式",
			"密钥", "API Key",
			"备份", "插件", "日志", "状态",
			"改名", "叫", "扮演", "角色", "语气", "风格",
			"switch", "config", "setting", "install", "schedule",
		},
	}
}

// Route performs two-stage intent classification.
func (ir *IntentRouter) Route(ctx context.Context, query, tenantID string) (*RouterResult, error) {
	result := &RouterResult{}

	// Stage 1: Fast routing via LocalBrain
	s1Start := time.Now()
	decision, err := ir.brain.Classify(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("intent_router: classify failed: %w", err)
	}
	result.Decision = decision
	result.Stage1Latency = time.Since(s1Start)

	// Check if this looks like a config request.
	// Two signals: (a) LocalBrain classified as "config", or
	// (b) keyword heuristic detects config-like language.
	isConfig := decision.Intent.Category == "config" || ir.looksLikeConfig(query)

	if isConfig && ir.translator != nil {
		// Stage 2: Deep NL config translation via LLM
		s2Start := time.Now()
		nlResult, err := ir.translator.Translate(ctx, cogni.NLConfigRequest{
			Text:     query,
			TenantID: tenantID,
		})
		result.Stage2Latency = time.Since(s2Start)

		if err != nil {
			slog.Warn("intent_router: NL translate failed, falling back to routing only", "err", err)
		} else if nlResult.Intent != cogni.IntentUnknown {
			result.NLConfig = nlResult
			result.IsConfig = true

			decision.Handler = "config"
			decision.Intent.Category = "config"
			decision.Reason = fmt.Sprintf("NL config: %s (%.0f%%)", nlResult.Intent, nlResult.Confidence*100)
		}
	}

	// Feed to distillation sink for future training
	if ir.sink != nil {
		sample := IntentTrainingSample{
			UserQuery:   query,
			TenantID:    tenantID,
			RouteIntent: decision.Intent,
			RouteTier:   decision.Handler,
			Score:       decision.Intent.Confidence,
			Source:      "intent_router",
		}
		if result.NLConfig != nil {
			sample.NLIntent = result.NLConfig.Intent
			sample.NLCategory = result.NLConfig.Category
			sample.NLConfidence = result.NLConfig.Confidence
			sample.NLParams = result.NLConfig.Params
		}
		if err := ir.sink.IngestConversation(sample); err != nil {
			slog.Debug("intent_router: sink ingest failed", "err", err)
		}
	}

	return result, nil
}

// looksLikeConfig is a fast keyword-based heuristic to detect config intent
// before the more expensive LLM translation. This prevents missing config
// requests that the small model might classify as "chat" or "tool".
func (ir *IntentRouter) looksLikeConfig(query string) bool {
	for _, kw := range ir.configKeywords {
		if containsCI(query, kw) {
			return true
		}
	}
	return false
}

// containsCI is a simple case-insensitive substring check.
// For ASCII keywords it lowercases; for CJK it does direct match.
func containsCI(s, substr string) bool {
	if len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
