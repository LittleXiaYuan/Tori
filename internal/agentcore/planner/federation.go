package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/opp"
)

// SetFederationBridge attaches the OPP federation bridge for A2A task delegation.
func (p *Planner) SetFederationBridge(fb FederationBridge) {
	delegationRuntime := p.ensureDelegationRuntime()
	delegationRuntime.SetFederationBridge(fb)
}

// FederationBridgeRef returns the current bridge (may be nil).
func (p *Planner) FederationBridgeRef() FederationBridge {
	if p == nil {
		return nil
	}
	delegationRuntime := p.ensureDelegationRuntime()
	return delegationRuntime.FederationBridge()
}

// federationToolDef returns the LLM function definition for `opp_delegate`.
// This tool allows the LLM to autonomously delegate sub-tasks to remote agents.
func federationToolDef() llm.FunctionDef {
	return llm.FunctionDef{
		Name:        "opp_delegate",
		Description: "将子任务委派给联邦网络中最合适的远程Agent。系统会根据模型能力、LoRA适配器和延迟自动路由。当本地Agent无法处理特定领域任务（如金融分析、代码审查、视觉理解）时使用此工具。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "要委派的任务描述（自然语言）",
				},
				"intent": map[string]any{
					"type":        "string",
					"description": "任务意图名称，如 code.review, data.analyze, finance.report",
				},
				"min_tier": map[string]any{
					"type":        "string",
					"description": "最低模型层级: fast, smart, expert",
					"enum":        []string{"fast", "smart", "expert"},
				},
				"features": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "所需模型能力: vision, code, function_calling, long_context",
				},
				"prefer_adapter": map[string]any{
					"type":        "string",
					"description": "优先选择的LoRA适配器领域: finance, legal, medical, code等",
				},
				"prefer_local": map[string]any{
					"type":        "boolean",
					"description": "是否优先使用本地模型（低延迟）",
				},
			},
			"required": []string{"task", "intent"},
		},
	}
}

// executeFederationDelegate handles the opp_delegate tool call within the FC loop.
func (p *Planner) executeFederationDelegate(ctx context.Context, args map[string]any, req PlanRequest) (string, error) {
	bridge := p.FederationBridgeRef()
	if bridge == nil {
		return "", fmt.Errorf("federation bridge not configured")
	}

	taskDesc, _ := args["task"].(string)
	intentName, _ := args["intent"].(string)
	minTier, _ := args["min_tier"].(string)
	preferAdapter, _ := args["prefer_adapter"].(string)
	preferLocal, _ := args["prefer_local"].(bool)

	var features []string
	if raw, ok := args["features"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				features = append(features, s)
			}
		}
	}

	dp := opp.DelegatePayload{
		Intent: opp.IntentEnvelope{
			Name:    intentName,
			Version: "1.0",
		},
		ModelRequirements: &opp.ModelRequirements{
			MinTier:       minTier,
			Features:      features,
			PreferLocal:   preferLocal,
			PreferAdapter: preferAdapter,
		},
		ContextMessages: []opp.DelegateMsg{
			{Role: "user", Content: taskDesc},
		},
	}

	if req.StepCallback != nil {
		evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan,
			fmt.Sprintf("🌐 正在联邦网络中寻找合适的Agent处理 [%s]...", intentName))
		evt.Meta.TenantID = req.TenantID
		req.StepCallback(evt)
	}

	result, err := bridge.Delegate(ctx, dp, 30*time.Second)
	if err != nil {
		slog.Warn("planner: federation delegation failed", "intent", intentName, "err", err)
		return "", err
	}

	if req.StepCallback != nil {
		evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult,
			fmt.Sprintf("远程Agent [%s] 完成任务 (模型: %s)", result.DelegatedTo, result.ModelUsed))
		evt.Meta.TenantID = req.TenantID
		req.StepCallback(evt)
	}

	out, _ := json.Marshal(map[string]any{
		"status":       result.Result.Status,
		"output":       result.Result.Output,
		"delegated_to": result.DelegatedTo,
		"model_used":   result.ModelUsed,
		"adapter_used": result.AdapterUsed,
	})
	return string(out), nil
}
