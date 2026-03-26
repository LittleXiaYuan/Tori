package ledger

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ledger"

	agtask "yunque-agent/internal/agentcore/task"
)

// MemoryBridge listens to task lifecycle events and automatically
// stores experiences into Ledger Memory when tasks complete or fail.
//
// This creates a learning loop:
//   - Task completes → positive experience stored
//   - Task fails → negative experience stored (with error reason)
//   - Planner runs → RecallBridge retrieves relevant experiences
type MemoryBridge struct {
	ldg       *ledger.Ledger
	taskStore agtask.Store
}

// NewMemoryBridge creates a bridge that stores task experiences in Ledger Memory.
func NewMemoryBridge(ldg *ledger.Ledger, ts agtask.Store) *MemoryBridge {
	return &MemoryBridge{ldg: ldg, taskStore: ts}
}

// OnEvent is a Runner.OnTaskEvent callback that stores experience memories
// when tasks reach terminal states.
func (mb *MemoryBridge) OnEvent(event, taskID, detail string) {
	switch event {
	case "task_completed":
		mb.storeExperience(taskID, true, detail)
	case "task_failed":
		mb.storeExperience(taskID, false, detail)
	}
}

func (mb *MemoryBridge) storeExperience(taskID string, success bool, detail string) {
	t, ok := mb.taskStore.Get(taskID)
	if !ok {
		return
	}

	ctx := context.Background()
	tenantID := t.TenantID
	if tenantID == "" {
		tenantID = "system"
	}

	// Build experience content
	var content string
	var confidence float64

	if success {
		content = mb.buildSuccessExperience(t, detail)
		confidence = 0.8
	} else {
		content = mb.buildFailureExperience(t, detail)
		confidence = 0.7
	}

	// Store as experience memory in Ledger
	taskIDRef := taskID
	err := mb.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		TenantID:   tenantID,
		TaskID:     &taskIDRef,
		Kind:       ledger.MemoryExperience,
		Key:        fmt.Sprintf("task_experience:%s", taskID),
		Content:    content,
		Source:     "extraction",
		Confidence: confidence,
	})
	if err != nil {
		slog.Warn("memory bridge: failed to store experience",
			"task", taskID, "err", err)
		return
	}

	status := "success"
	if !success {
		status = "failure"
	}
	slog.Debug("memory bridge: stored experience",
		"task", taskID, "status", status, "tenant", tenantID)
}

func (mb *MemoryBridge) buildSuccessExperience(t *agtask.Task, detail string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务成功: %s\n", t.Description))

	if len(t.Steps) > 0 {
		sb.WriteString("执行步骤:\n")
		for i, step := range t.Steps {
			result := step.Result
			if len(result) > 100 {
				result = result[:100] + "..."
			}
			sb.WriteString(fmt.Sprintf("  %d. %s → %s\n", i+1, step.Action, result))
		}
	}

	if detail != "" {
		sb.WriteString(fmt.Sprintf("完成信息: %s\n", detail))
	}

	return sb.String()
}

func (mb *MemoryBridge) buildFailureExperience(t *agtask.Task, detail string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务失败: %s\n", t.Description))

	if t.Error != "" {
		sb.WriteString(fmt.Sprintf("错误原因: %s\n", t.Error))
	}

	// Find the failed step
	for _, step := range t.Steps {
		if step.Status == agtask.StepFailed {
			sb.WriteString(fmt.Sprintf("失败步骤: %s (重试 %d 次)\n", step.Action, step.RetryCount))
			break
		}
	}

	if detail != "" {
		sb.WriteString(fmt.Sprintf("详情: %s\n", detail))
	}

	sb.WriteString("注意: 下次遇到类似任务时应避免此方法或采用替代方案。\n")

	return sb.String()
}
