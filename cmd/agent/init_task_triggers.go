package main

import (
	"context"
	"fmt"

	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/pkg/skills"
)

func wireTriggerExecutor(
	exec *trigger.Executor,
	taskStore task.Store,
	taskRunner *task.Runner,
	threadMgr *task.ThreadManager,
	channelReg *channel.Registry,
	app *agentrt.App,
	costAwareLLM func(ctx context.Context, system, user string) (string, error),
	taskEngineCtx context.Context,
) {
	exec.SetCreateTask(func(ctx context.Context, tenantID, title, desc string) (string, error) {
		t, err := taskStore.Create(task.CreateRequest{Title: title, Description: desc, TenantID: tenantID})
		if err != nil {
			return "", err
		}
		go taskRunner.Run(taskEngineCtx, t.ID)
		return t.ID, nil
	})
	exec.SetContinueTask(func(ctx context.Context, taskID, message string) error {
		if threadMgr != nil {
			threadMgr.Post(taskID, "", "trigger", message)
		}
		return taskRunner.Resume(ctx, taskID)
	})
	exec.SetSendMessage(func(ctx context.Context, channelID, threadID, message string) (string, error) {
		for _, ch := range channelReg.All() {
			if ch.Type() == channelID {
				target := threadID
				if target == "" {
					target = channelID
				}
				if err := ch.Send(ctx, target, channel.Reply{Content: message}); err != nil {
					return "", err
				}
				return "sent", nil
			}
		}
		return "", fmt.Errorf("channel not found: %s", channelID)
	})
	exec.SetCallSkill(func(ctx context.Context, skillName string, args map[string]any) (string, float64, error) {
		sk, ok := app.SkillRegistry.Get(skillName)
		if !ok {
			return "", 0, fmt.Errorf("skill not found: %s", skillName)
		}
		env := &skills.Environment{LLMCall: costAwareLLM}
		result, err := sk.Execute(ctx, args, env)
		return result, 0, err
	})
	exec.SetWriteMemory(func(ctx context.Context, tenantID, content string) error {
		return app.Orchestrator.Ingest(ctx, tenantID, content, "trigger", "trigger_action")
	})
	exec.SetUpdateProfile(func(ctx context.Context, tenantID, key, value string) error {
		if app.EditableMem != nil {
			app.EditableMem.AddBlock(key, value, 2000)
			return nil
		}
		return fmt.Errorf("editable memory not available")
	})
}
