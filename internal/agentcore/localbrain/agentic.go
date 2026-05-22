package localbrain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	ldg "yunque-agent/internal/ledgercore"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/jsonutil"
)

// AgenticThinking 实现"边想边做"范式。
//
// 核心思想（来自林俊旸的 Reasoning→Agentic 论述）：
//   - 传统 Chain-of-Thought：想完再做（thinking → action → observation → thinking ...）
//   - Agentic Thinking：思考本身是为了行动，行动的结果实时塑造思考
//
// 技术实现：
//  1. 小模型做快速"直觉判断"（<50ms），决定是否需要深度思考
//  2. 思考强度根据任务复杂度动态调整（不是每次都 deep think）
//  3. 每一步 action 的结果立即融入下一步 thinking 的 context
//  4. Ledger 完整记录 thinking trajectory，用于事后 LoRA 微调
type AgenticThinking struct {
	brain  *LocalBrain
	pool   *llm.Pool
	ledger *ldg.Ledger

	config AgenticConfig
}

// AgenticConfig 控制思考行为。
type AgenticConfig struct {
	// 快速直觉判断的最大时间（超时则跳过直觉，直接深度思考）
	IntuitionTimeout time.Duration `json:"intuition_timeout"`

	// 思考深度层级
	// Level 0: 不思考，直接执行（简单命令）
	// Level 1: 快速思考（~1 sentence）
	// Level 2: 标准思考（plan+reason）
	// Level 3: 深度思考（多角度分析+假设检验）
	DefaultThinkLevel int `json:"default_think_level"`

	// 思考步骤上限
	MaxThinkSteps int `json:"max_think_steps"`

	// 是否启用 interleaved thinking（交错思考+执行）
	InterleavedMode bool `json:"interleaved_mode"`
}

// DefaultAgenticConfig 返回默认配置。
func DefaultAgenticConfig() AgenticConfig {
	return AgenticConfig{
		IntuitionTimeout:  50 * time.Millisecond,
		DefaultThinkLevel: 1,
		MaxThinkSteps:     10,
		InterleavedMode:   true,
	}
}

// ThinkLevel 思考深度。
type ThinkLevel int

const (
	ThinkNone   ThinkLevel = 0 // 不思考
	ThinkQuick  ThinkLevel = 1 // 一句话思考
	ThinkNormal ThinkLevel = 2 // 标准 plan
	ThinkDeep   ThinkLevel = 3 // 多角度深度分析
)

// ThinkRequest 请求。
type ThinkRequest struct {
	TaskID   string
	TenantID string
	Query    string
	// 上一步 action 的结果（Agentic Thinking 核心：action 塑造 thinking）
	PrevActionResult string
	// 当前已执行的步骤数
	StepIndex int
	// 所有历史步骤的摘要
	StepHistory []StepSummary
}

// StepSummary 单步骤摘要。
type StepSummary struct {
	Action  string `json:"action"`
	Result  string `json:"result"`
	Success bool   `json:"success"`
}

// ThinkResult 结果。
type ThinkResult struct {
	Level      ThinkLevel `json:"level"`
	Thought    string     `json:"thought"`
	NextAction string     `json:"next_action"` // 建议的下一步 action
	Confidence float64    `json:"confidence"`
	ShouldStop bool       `json:"should_stop"` // 是否应该停止（任务已完成）
}

// NewAgenticThinking 创建 Agentic Thinking 引擎。
func NewAgenticThinking(brain *LocalBrain, pool *llm.Pool, ledger *ldg.Ledger, cfg ...AgenticConfig) *AgenticThinking {
	config := DefaultAgenticConfig()
	if len(cfg) > 0 {
		config = cfg[0]
	}
	return &AgenticThinking{
		brain:  brain,
		pool:   pool,
		ledger: ledger,
		config: config,
	}
}

// Think 执行一次"agentic thinking"步骤。
// 与传统 CoT 的关键区别：PrevActionResult 会实时影响思考方向。
func (at *AgenticThinking) Think(ctx context.Context, req ThinkRequest) (*ThinkResult, error) {
	// 第一步：用小模型做"直觉判断"——决定思考深度
	level := at.determineThinkLevel(ctx, req)

	// 记录推理痕迹到 Ledger
	if at.ledger != nil {
		tracer := at.ledger.Reasoning(req.TaskID, "agentic-thinking")
		tracer.Think(ctx, fmt.Sprintf("[level=%d] determining depth for step %d", level, req.StepIndex), nil)
	}

	switch level {
	case ThinkNone:
		return &ThinkResult{Level: ThinkNone, Confidence: 0.9}, nil
	case ThinkQuick:
		return at.quickThink(ctx, req)
	case ThinkNormal:
		return at.normalThink(ctx, req)
	case ThinkDeep:
		return at.deepThink(ctx, req)
	}

	return at.normalThink(ctx, req)
}

// determineThinkLevel 用小模型快速判断需要多深的思考。
func (at *AgenticThinking) determineThinkLevel(ctx context.Context, req ThinkRequest) ThinkLevel {
	if at.brain == nil || at.brain.client == nil {
		return ThinkLevel(at.config.DefaultThinkLevel)
	}

	// 快速路径：第一步总是至少 normal think
	if req.StepIndex == 0 {
		return ThinkNormal
	}

	// 快速路径：上一步失败 → 深度思考（需要重新规划）
	if len(req.StepHistory) > 0 && !req.StepHistory[len(req.StepHistory)-1].Success {
		return ThinkDeep
	}

	// 快速路径：步骤数过多 → 深度思考（可能陷入循环）
	if req.StepIndex > at.config.MaxThinkSteps/2 {
		return ThinkDeep
	}

	// 用小模型判断
	timeoutCtx, cancel := context.WithTimeout(ctx, at.config.IntuitionTimeout)
	defer cancel()

	decision, err := at.brain.Classify(timeoutCtx, req.Query, req.TenantID)
	if err != nil {
		return ThinkLevel(at.config.DefaultThinkLevel)
	}

	switch decision.Intent.Complexity {
	case "simple":
		return ThinkQuick
	case "hard":
		return ThinkDeep
	default:
		return ThinkNormal
	}
}

// quickThink 快速思考：一句话判断下一步。
func (at *AgenticThinking) quickThink(ctx context.Context, req ThinkRequest) (*ThinkResult, error) {
	var client *llm.Client
	if at.brain != nil {
		client = at.brain.client
	}
	if client == nil && at.pool != nil {
		client = at.pool.GetOrFallback("fast")
	}
	if client == nil {
		return &ThinkResult{Level: ThinkQuick, Confidence: 0.5, Thought: "no client available"}, nil
	}

	prompt := fmt.Sprintf("Based on the task and previous result, what's the next action? Reply in one sentence.\nTask: %s\nPrev result: %s",
		truncate(req.Query, 200), truncate(req.PrevActionResult, 200))

	msgs := []llm.Message{
		{Role: "system", Content: "You are a quick action planner. Be extremely concise."},
		{Role: "user", Content: prompt},
	}

	reply, err := client.Chat(ctx, msgs, 0.3)
	if err != nil {
		slog.Warn("agentic: quickThink failed", "err", err)
		return &ThinkResult{Level: ThinkQuick, Confidence: 0.5, Thought: "quick think failed, proceeding with default"}, nil
	}

	if at.ledger != nil {
		tracer := at.ledger.Reasoning(req.TaskID, "agentic-thinking")
		tracer.Think(ctx, "[quick] "+reply, nil)
	}

	return &ThinkResult{
		Level:      ThinkQuick,
		Thought:    reply,
		Confidence: 0.7,
	}, nil
}

// normalThink 标准思考：plan + reason。
func (at *AgenticThinking) normalThink(ctx context.Context, req ThinkRequest) (*ThinkResult, error) {
	client := at.selectClient("smart")
	if client == nil {
		return nil, fmt.Errorf("normalThink: no LLM client available")
	}

	// 构建包含历史步骤的 context
	historyStr := at.formatHistory(req.StepHistory)

	prompt := fmt.Sprintf(`Task: %s

Previous steps:
%s

Last action result: %s

Think step by step:
1. What have we accomplished so far?
2. What should we do next?
3. Are we done?

Reply as JSON: {"thought":"...","next_action":"...","confidence":0.0-1.0,"should_stop":true|false}`,
		req.Query, historyStr, truncate(req.PrevActionResult, 500))

	msgs := []llm.Message{
		{Role: "system", Content: "You are an agentic planner. Think FOR action, not just think then act. Your thinking should directly produce actionable next steps."},
		{Role: "user", Content: prompt},
	}

	reply, err := client.Chat(ctx, msgs, 0.5)
	if err != nil {
		return nil, fmt.Errorf("normalThink: %w", err)
	}

	result := &ThinkResult{Level: ThinkNormal}
	if err := jsonutil.Unmarshal(reply, result); err != nil {
		result.Thought = reply
		result.Confidence = 0.5
	}

	if at.ledger != nil {
		tracer := at.ledger.Reasoning(req.TaskID, "agentic-thinking")
		tracer.Think(ctx, "[normal] "+result.Thought, map[string]interface{}{
			"confidence": result.Confidence,
			"next":       result.NextAction,
		})
	}

	return result, nil
}

// deepThink 深度思考：多角度分析 + 假设检验。
func (at *AgenticThinking) deepThink(ctx context.Context, req ThinkRequest) (*ThinkResult, error) {
	client := at.selectClient("expert")
	if client == nil {
		return nil, fmt.Errorf("deepThink: no LLM client available")
	}

	historyStr := at.formatHistory(req.StepHistory)

	prompt := fmt.Sprintf(`Task: %s

Full execution history:
%s

Last result: %s

This task requires deep analysis. Consider:
1. Are we on the right track, or should we backtrack?
2. Are there alternative approaches we haven't tried?
3. What could go wrong with the current plan?
4. What's the most efficient next step?

Reply as JSON: {"thought":"...","next_action":"...","confidence":0.0-1.0,"should_stop":true|false}`,
		req.Query, historyStr, truncate(req.PrevActionResult, 1000))

	msgs := []llm.Message{
		{Role: "system", Content: "You are a deep reasoning engine. Analyze the situation from multiple angles before recommending action. If stuck, suggest backtracking."},
		{Role: "user", Content: prompt},
	}

	reply, err := client.Chat(ctx, msgs, 0.7)
	if err != nil {
		return nil, fmt.Errorf("deepThink: %w", err)
	}

	result := &ThinkResult{Level: ThinkDeep}
	if err := jsonutil.Unmarshal(reply, result); err != nil {
		result.Thought = reply
		result.Confidence = 0.4
	}

	if at.ledger != nil {
		tracer := at.ledger.Reasoning(req.TaskID, "agentic-thinking")
		tracer.Think(ctx, "[deep] "+result.Thought, map[string]interface{}{
			"confidence": result.Confidence,
			"next":       result.NextAction,
			"level":      "deep",
		})
		if result.Confidence < 0.3 {
			tracer.Backtrack(ctx, "low confidence after deep think", result.NextAction, nil)
		}
	}

	return result, nil
}

// selectClient 选择合适的 LLM client。
func (at *AgenticThinking) selectClient(tier string) *llm.Client {
	if at.pool != nil {
		if c := at.pool.Get(tier); c != nil {
			return c
		}
		return at.pool.Primary()
	}
	if at.brain != nil && at.brain.client != nil {
		return at.brain.client
	}
	return nil
}

// formatHistory 格式化步骤历史。
func (at *AgenticThinking) formatHistory(steps []StepSummary) string {
	if len(steps) == 0 {
		return "(no steps executed yet)"
	}
	var result string
	for i, s := range steps {
		status := "✓"
		if !s.Success {
			status = "✗"
		}
		result += fmt.Sprintf("%d. [%s] %s → %s\n", i+1, status, s.Action, truncate(s.Result, 100))
	}
	return result
}
