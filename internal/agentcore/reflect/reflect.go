package reflect

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"yunque-agent/internal/agentcore/llm"
)

// ReflectMode controls how the reflect engine behaves.
const (
	ModeStrict   = "strict"   // block and retry on unsatisfied
	ModeLearning = "learning" // async eval, never block (default)
	ModeOff      = "off"      // completely disabled
)

// Engine evaluates execution results and suggests improvements.
type Engine struct {
	mu        sync.RWMutex
	llm       *llm.Client // primary LLM (fallback)
	evalPool  *llm.Pool   // multi-model pool for eval
	evalModel string      // pool key for eval model (empty = use primary)
	enabled   bool        // global on/off
	mode      string      // "strict" | "learning" | "off"
}

// NewEngine creates a reflection engine.
func NewEngine(llmClient *llm.Client) *Engine {
	return &Engine{
		llm:     llmClient,
		enabled: true,
		mode:    ModeLearning,
	}
}

// SetEvalModel configures a separate model from the LLM pool for evaluation.
// This prevents the evaluator from competing with the main LLM for resources.
func (e *Engine) SetEvalModel(pool *llm.Pool, modelKey string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evalPool = pool
	e.evalModel = modelKey
	slog.Info("reflect: eval model configured", "key", modelKey)
}

// SetEnabled toggles the engine on/off at runtime (e.g. from Web UI).
func (e *Engine) SetEnabled(on bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = on
}

// SetMode sets the reflect mode: "strict", "learning", or "off".
func (e *Engine) SetMode(mode string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch mode {
	case ModeStrict, ModeLearning, ModeOff:
		e.mode = mode
	default:
		e.mode = ModeLearning
	}
	slog.Info("reflect: mode changed", "mode", e.mode)
}

// Enabled returns whether the engine is enabled.
func (e *Engine) Enabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled && e.mode != ModeOff
}

// Mode returns the current reflect mode.
func (e *Engine) Mode() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.mode
}

// evalClient returns the LLM client to use for evaluation.
// Prefers the configured eval model from the pool; falls back to primary.
func (e *Engine) evalClient() *llm.Client {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.evalPool != nil && e.evalModel != "" {
		if c := e.evalPool.Get(e.evalModel); c != nil {
			return c
		}
	}
	return e.llm
}

// Evaluation is the result of a reflection.
type Evaluation struct {
	Satisfied     bool           `json:"satisfied"`
	Quality       int            `json:"quality"` // 1-10
	Issues        []string       `json:"issues"`
	Suggestions   []string       `json:"suggestions"`
	MemoryUpdates []MemoryUpdate `json:"memory_updates,omitempty"`
}

// MemoryUpdate is a suggested change to the memory store.
type MemoryUpdate struct {
	Action string `json:"action"` // "add", "update", "delete"
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// Evaluate assesses whether the agent's response satisfies the user's intent.
func (e *Engine) Evaluate(ctx context.Context, userIntent, agentReply string, skillResults []string) (*Evaluation, error) {
	if !e.Enabled() {
		return &Evaluation{Satisfied: true, Quality: 8}, nil
	}

	prompt := fmt.Sprintf(`你是一个AI执行质量评估器。请评估以下AI回复是否满足用户意图。

## 用户意图
%s

## AI回复
%s

## 技能执行结果
%v

请以JSON格式输出评估结果：
{
  "satisfied": true/false,
  "quality": 1-10,
  "issues": ["问题1", ...],
  "suggestions": ["建议1", ...],
  "memory_updates": [{"action": "add/update/delete", "key": "...", "value": "..."}]
}

只输出JSON，不要其他内容。`, userIntent, agentReply, skillResults)

	client := e.evalClient()
	reply, err := client.Chat(ctx, []llm.Message{
		{Role: "system", Content: "你是质量评估器，只输出JSON。"},
		{Role: "user", Content: prompt},
	}, 0.1)
	if err != nil {
		return nil, fmt.Errorf("reflect: %w", err)
	}

	var eval Evaluation
	if err := json.Unmarshal([]byte(extractJSON(reply)), &eval); err != nil {
		// Default to satisfied if parsing fails
		return &Evaluation{Satisfied: true, Quality: 7}, nil
	}
	return &eval, nil
}

// ShouldRetry returns true if the evaluation suggests retrying.
func (e *Evaluation) ShouldRetry() bool {
	return !e.Satisfied && e.Quality < 5
}

func extractJSON(s string) string {
	start := -1
	for i, c := range s {
		if c == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return "{}"
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return "{}"
}
