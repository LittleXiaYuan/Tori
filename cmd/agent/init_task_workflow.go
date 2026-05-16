package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/config"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/skills"
	"yunque-agent/plugins/general"
)

func wireWorkflowExecutors(gw *gateway.Gateway, wfEngine *workflow.Engine, knowledgeStore *knowledge.Store, cfg *config.Config) {
	wfEngine.SetBrowserExecutor(func(ctx context.Context, action string, args map[string]any) (string, error) {
		hub := gw.BrowserHub()
		if hub == nil || !hub.Connected() {
			return "", fmt.Errorf("browser extension not connected; install and connect the Yunque Browser Connector extension")
		}
		action = strings.TrimSpace(action)
		if action == "" {
			action = "navigate"
		}
		if strings.HasPrefix(action, "browser_") || action == "session_status" {
			// already normalized
		} else {
			action = "browser_" + action
		}
		browserAction := map[string]any{"type": action}
		if target, ok := args["target"].(string); ok {
			if action == "browser_navigate" {
				browserAction["url"] = target
			} else {
				browserAction["target"] = map[string]any{"strategy": "bySelector", "selector": target}
			}
		}
		if sel, ok := args["selector"].(string); ok && sel != "" {
			browserAction["target"] = map[string]any{"strategy": "bySelector", "selector": sel}
		}
		if label, ok := args["text_target"].(string); ok && label != "" {
			browserAction["target"] = map[string]any{"strategy": "byText", "text": label}
		}
		if text, ok := args["text"].(string); ok {
			browserAction["text"] = text
		}
		if pressEnter, ok := args["press_enter"].(bool); ok {
			browserAction["press_enter"] = pressEnter
		}
		actionData, _ := json.Marshal(browserAction)
		resultData, err := hub.SendActionRaw(ctx, actionData)
		if err != nil {
			return "", err
		}
		return string(resultData), nil
	})

	sandboxRunner, sandboxErr := sandbox.NewRunner(sandbox.SandboxConfig{
		BaseDir: filepath.Join(cfg.DataDir, "sandbox"),
		Policy:  sandbox.DefaultPolicy(),
	})
	if sandboxErr != nil {
		slog.Warn("sandbox runner init failed, code nodes will be unavailable", "err", sandboxErr)
	} else {
		wfEngine.SetCodeExecutor(func(ctx context.Context, language, code string) (string, error) {
			res, err := sandboxRunner.Run(ctx, sandbox.RunRequest{
				Language: language,
				Code:     code,
				Timeout:  30 * time.Second,
			})
			if err != nil {
				return "", err
			}
			if res.ExitCode != 0 {
				return "", fmt.Errorf("exit %d: %s", res.ExitCode, res.Stderr)
			}
			return res.Stdout, nil
		})
		slog.Info("workflow code executor wired", "backend", sandboxRunner.Type())
	}

	wfEngine.SetKnowledgeExecutor(func(ctx context.Context, query string, topK int) (string, error) {
		scored := knowledgeStore.HybridSearchReranked(ctx, query, topK)
		if len(scored) == 0 {
			return "未找到匹配的知识条目", nil
		}
		var buf strings.Builder
		for i, sc := range scored {
			fmt.Fprintf(&buf, "[%d] (score %.2f) %s\n", i+1, sc.Score, sc.Chunk.Content)
		}
		return buf.String(), nil
	})
	slog.Info("workflow executors wired", "browser", "lazy", "knowledge", "ready")
}

func wireWorkflowSkills(
	app *agentrt.App,
	gw *gateway.Gateway,
	wfStore *workflow.JSONStore,
	wfEngine *workflow.Engine,
	triggerRT *trigger.Runtime,
	costAwareLLM func(ctx context.Context, system, user string) (string, error),
	taskEngineCtx context.Context,
) {
	for _, pl := range app.PluginReg.All() {
		if gp, ok := pl.(*general.GeneralPlugin); ok {
			gp.SetWorkflowStore(wfStore)
			break
		}
	}
	app.SkillRegistry = skills.NewRegistry()
	for _, s := range app.PluginReg.AllSkills() {
		app.SkillRegistry.Register(s)
	}
	slog.Info("generate_workflow skill: shared workflow store injected via plugin")

	var lastPlanCache sync.Map
	gw.SetLastPlanCache(&lastPlanCache)

	saveWFSkill := workflow.NewSaveWorkflowSkill(wfStore, func(tenantID string) *planner.PlanResult {
		if v, ok := lastPlanCache.Load(tenantID); ok {
			return v.(*planner.PlanResult)
		}
		return nil
	})
	saveWFSkill.SetTriggerBinder(func(wfID, triggerExpr, tenantID string) (string, error) {
		tType, tValue := trigger.ParseTriggerExpr(triggerExpr)
		tID := triggerRT.Register(trigger.Trigger{
			Name:   "auto:" + wfID,
			Kind:   tType,
			Event:  trigger.EventName(tValue),
			Action: trigger.Action{Type: trigger.ActionRunWorkflow, Data: map[string]any{"workflow_id": wfID}},
		})
		return tID, nil
	})
	app.SkillRegistry.Register(saveWFSkill)

	defaultTID := os.Getenv("DEFAULT_TENANT_ID")
	if defaultTID == "" {
		defaultTID = "default"
	}

	runWFSkill := workflow.NewRunWorkflowSkill(wfStore, func(ctx context.Context, instanceID string) error {
		go wfEngine.Run(taskEngineCtx, instanceID)
		return nil
	})
	runWFSkill.SetTenantID(defaultTID)
	app.SkillRegistry.Register(runWFSkill)

	listWFSkill := workflow.NewListWorkflowsSkill(wfStore)
	listWFSkill.SetTenantID(defaultTID)
	app.SkillRegistry.Register(listWFSkill)
}
