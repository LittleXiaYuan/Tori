package task

import (
	"encoding/json"
	"fmt"
)

// ──────────────────────────────────────────────
// Card builders for Feishu Interactive Cards
//
// These produce raw JSON strings compatible with Feishu's
// interactive card format. They are self-contained and do not
// depend on the channel package to avoid circular imports.
// ──────────────────────────────────────────────

// buildStepCard builds a Feishu interactive card for step progress.
func buildStepCard(taskTitle string, stepID, totalSteps int, action, status, detail string) string {
	var (
		color string
		emoji string
		label string
	)
	switch status {
	case "failed":
		color = "red"
		emoji = "❌"
		label = "失败"
	default:
		color = "blue"
		emoji = "✅"
		label = "完成"
	}

	title := fmt.Sprintf("%s 步骤 %d/%d %s", emoji, stepID, totalSteps, label)

	elements := []any{}

	// Progress bar
	if totalSteps > 0 {
		done := stepID
		if status == "failed" {
			done = stepID - 1
		}
		bar := textProgressBar(done, totalSteps)
		elements = append(elements, mdBlock(fmt.Sprintf("**进度** %s  %d/%d", bar, done, totalSteps)))
	}

	elements = append(elements, hrBlock())
	elements = append(elements, mdBlock(fmt.Sprintf("**任务** %s", taskTitle)))
	elements = append(elements, mdBlock(fmt.Sprintf("**步骤** %s", action)))

	if detail != "" {
		if len(detail) > 500 {
			detail = detail[:500] + "..."
		}
		elements = append(elements, hrBlock())
		elements = append(elements, mdBlock(detail))
	}

	return buildCardJSON(title, color, elements)
}

// buildTaskCompletedCard builds a success card.
func buildTaskCompletedCard(taskTitle, taskID, summary string) string {
	elements := []any{
		mdBlock(fmt.Sprintf("**%s**", taskTitle)),
	}
	if summary != "" {
		if len(summary) > 1000 {
			summary = summary[:1000] + "..."
		}
		elements = append(elements, hrBlock())
		elements = append(elements, mdBlock(summary))
	}
	elements = append(elements, hrBlock())
	elements = append(elements, noteBlock(fmt.Sprintf("任务 ID: %s", taskID)))

	return buildCardJSON("🎉 任务完成", "green", elements)
}

// buildTaskFailedCard builds a failure card with retry button.
func buildTaskFailedCard(taskTitle, taskID, errMsg string) string {
	elements := []any{
		mdBlock(fmt.Sprintf("**%s**", taskTitle)),
	}
	if errMsg != "" {
		if len(errMsg) > 800 {
			errMsg = errMsg[:800] + "..."
		}
		elements = append(elements, hrBlock())
		elements = append(elements, mdBlock(fmt.Sprintf("```\n%s\n```", errMsg)))
	}
	elements = append(elements, hrBlock())
	elements = append(elements, actionBlock(
		buttonEl("重试", "primary", map[string]string{"action": "retry_task", "task_id": taskID}),
		buttonEl("取消", "danger", map[string]string{"action": "cancel_task", "task_id": taskID}),
	))
	elements = append(elements, noteBlock(fmt.Sprintf("任务 ID: %s", taskID)))

	return buildCardJSON("💥 任务失败", "red", elements)
}

// ── low-level card JSON helpers ────────────────

func buildCardJSON(title, color string, elements []any) string {
	card := map[string]any{
		"header": map[string]any{
			"title": map[string]string{
				"tag":     "plain_text",
				"content": title,
			},
			"template": color,
		},
		"elements": elements,
	}
	data, _ := json.Marshal(card)
	return string(data)
}

func mdBlock(content string) map[string]any {
	return map[string]any{
		"tag":     "markdown",
		"content": content,
	}
}

func hrBlock() map[string]string {
	return map[string]string{"tag": "hr"}
}

func noteBlock(text string) map[string]any {
	return map[string]any{
		"tag": "note",
		"elements": []map[string]string{
			{"tag": "plain_text", "content": text},
		},
	}
}

func actionBlock(buttons ...map[string]any) map[string]any {
	actions := make([]any, len(buttons))
	for i, b := range buttons {
		actions[i] = b
	}
	return map[string]any{
		"tag":     "action",
		"actions": actions,
	}
}

func buttonEl(text, style string, value map[string]string) map[string]any {
	btn := map[string]any{
		"tag": "button",
		"text": map[string]string{
			"tag":     "plain_text",
			"content": text,
		},
		"value": value,
	}
	if style != "" {
		btn["type"] = style
	}
	return btn
}

// textProgressBar generates a unicode progress bar.
func textProgressBar(done, total int) string {
	if total <= 0 {
		return ""
	}
	const width = 10
	filled := done * width / total
	if filled > width {
		filled = width
	}
	bar := make([]byte, width)
	for i := range bar {
		if i < filled {
			bar[i] = '#'
		} else {
			bar[i] = '-'
		}
	}
	return "[" + string(bar) + "]"
}
