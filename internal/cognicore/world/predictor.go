package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/pkg/jsonutil"
)

// LLMFunc abstracts an LLM call for impact analysis.
type LLMFunc func(ctx context.Context, system, user string) (string, error)

// Predictor uses both the world model state and LLM reasoning
// to predict the real-world impact of agent actions.
type Predictor struct {
	model   *Model
	llmCall LLMFunc
	mu      sync.Mutex
	history []PredictionRecord
}

// PredictionRecord logs a prediction and its actual outcome for learning.
type PredictionRecord struct {
	Action        string            `json:"action"`
	Prediction    *ImpactPrediction `json:"prediction"`
	ActualOutcome string            `json:"actual_outcome,omitempty"`
	Accurate      *bool             `json:"accurate,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
}

// RippleEffect describes a cascading consequence of an action.
type RippleEffect struct {
	Key         string  `json:"key"`
	Effect      string  `json:"effect"`
	Probability float64 `json:"probability"`
	Severity    string  `json:"severity"` // low/medium/high/critical
	Depth       int     `json:"depth"`    // 1 = direct, 2+ = cascading
}

// DeepImpact is an LLM-enhanced impact prediction with ripple effects.
type DeepImpact struct {
	Action         string         `json:"action"`
	DirectEffects  []RippleEffect `json:"direct_effects"`
	Cascading      []RippleEffect `json:"cascading_effects"`
	RiskLevel      string         `json:"risk_level"`
	RiskScore      float64        `json:"risk_score"`     // 0.0~1.0
	Recommendation string         `json:"recommendation"` // proceed/caution/block
	Reasoning      string         `json:"reasoning"`
}

// NewPredictor creates an impact predictor.
func NewPredictor(model *Model, llmCall LLMFunc) *Predictor {
	return &Predictor{
		model:   model,
		llmCall: llmCall,
	}
}

// PredictDeep uses LLM to analyze potential impacts, including cascading effects.
func (p *Predictor) PredictDeep(ctx context.Context, action string, targetKeys []string) (*DeepImpact, error) {
	// 第一步：用 World Model 的结构化数据做基础分析
	basePred := p.model.PredictImpact(action, targetKeys)

	// 第二步：构建 LLM 分析所需的上下文
	stateContext := p.buildStateContext(targetKeys)

	if p.llmCall == nil {
		// 无 LLM 时退回基础预测
		return p.convertBasicPrediction(basePred), nil
	}

	system := `You are a world model impact analyzer. Given the current state and a proposed action, predict:
1. Direct effects on the target
2. Cascading ripple effects on dependencies
3. Overall risk assessment

Output JSON:
{"direct_effects":[{"key":"...","effect":"...","probability":0.9,"severity":"low|medium|high|critical","depth":1}],
 "cascading_effects":[{"key":"dep_key","effect":"...","probability":0.5,"severity":"medium","depth":2}],
 "risk_level":"low|medium|high|critical",
 "risk_score":0.0-1.0,
 "recommendation":"proceed|caution|block",
 "reasoning":"..."}`

	user := fmt.Sprintf("Action: %s\n\nCurrent state:\n%s\n\nBase analysis: %d direct targets, risk=%s",
		action, stateContext, len(basePred.AffectedKeys), basePred.RiskLevel)

	reply, err := p.llmCall(ctx, system, user)
	if err != nil {
		slog.Warn("world predictor: LLM failed, using basic", "err", err)
		return p.convertBasicPrediction(basePred), nil
	}

	impact := &DeepImpact{Action: action}
	if err := json.Unmarshal([]byte(jsonutil.Extract(reply)), impact); err != nil {
		slog.Warn("world predictor: parse failed", "err", err)
		return p.convertBasicPrediction(basePred), nil
	}

	// 记录预测历史（用于准确度跟踪）
	p.mu.Lock()
	p.history = append(p.history, PredictionRecord{
		Action: action, Prediction: basePred, Timestamp: time.Now(),
	})
	if len(p.history) > 500 {
		p.history = p.history[len(p.history)-500:]
	}
	p.mu.Unlock()

	return impact, nil
}

// RecordOutcome records the actual outcome of an action for prediction accuracy tracking.
func (p *Predictor) RecordOutcome(action, outcome string, accurate bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := len(p.history) - 1; i >= 0; i-- {
		if p.history[i].Action == action && p.history[i].ActualOutcome == "" {
			p.history[i].ActualOutcome = outcome
			p.history[i].Accurate = &accurate
			break
		}
	}
}

// Accuracy returns the prediction accuracy rate.
func (p *Predictor) Accuracy() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	total, correct := 0, 0
	for _, r := range p.history {
		if r.Accurate != nil {
			total++
			if *r.Accurate {
				correct++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(correct) / float64(total)
}

// buildStateContext formats relevant world state for LLM context.
func (p *Predictor) buildStateContext(keys []string) string {
	var ctx string
	for _, key := range keys {
		if s, ok := p.model.Get(key); ok {
			deps := ""
			if len(s.Dependencies) > 0 {
				deps = fmt.Sprintf(" [deps: %v]", s.Dependencies)
			}
			ctx += fmt.Sprintf("- %s (%s): %s (confidence=%.2f, age=%s)%s\n",
				s.Key, s.Kind, truncateStr(s.Value, 100), s.Confidence,
				time.Since(s.LastVerified).Round(time.Minute), deps)
		}
	}
	if ctx == "" {
		ctx = "(no known state for target keys)"
	}
	return ctx
}

// convertBasicPrediction converts a basic ImpactPrediction to DeepImpact.
func (p *Predictor) convertBasicPrediction(pred *ImpactPrediction) *DeepImpact {
	impact := &DeepImpact{
		Action:         pred.Action,
		RiskLevel:      pred.RiskLevel,
		RiskScore:      riskToScore(pred.RiskLevel),
		Recommendation: "proceed",
		Reasoning:      "basic prediction (no LLM available)",
	}
	for _, pp := range pred.Predictions {
		impact.DirectEffects = append(impact.DirectEffects, RippleEffect{
			Key:         pp.Key,
			Effect:      pp.PredictedValue,
			Probability: pp.Confidence,
			Severity:    pred.RiskLevel,
			Depth:       1,
		})
	}
	if impact.RiskScore > 0.7 {
		impact.Recommendation = "caution"
	}
	return impact
}

func riskToScore(level string) float64 {
	switch level {
	case "critical":
		return 1.0
	case "high":
		return 0.8
	case "medium":
		return 0.5
	default:
		return 0.2
	}
}

func truncateStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
