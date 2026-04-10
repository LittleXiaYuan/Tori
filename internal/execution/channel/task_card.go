package channel

import "fmt"

// ──────────────────────────────────────────────
// Task Progress Cards for Feishu/Lark
//
// These builders create rich interactive cards for
// step completion, task result, and task failure events.
// ──────────────────────────────────────────────

// TaskStepCard builds a card for step completion/failure.
//
//	taskTitle: human-readable task name
//	stepID:    1-based step number
//	total:     total steps in the plan
//	action:    step description
//	status:    "done" | "failed"
//	detail:    result text or error message (truncated if long)
func TaskStepCard(taskTitle string, stepID, total int, action, status, detail string) *Card {
	var (
		color CardColor
		emoji string
		label string
	)
	switch status {
	case "failed":
		color = CardRed
		emoji = "❌"
		label = "失败"
	default:
		color = CardBlue
		emoji = "✅"
		label = "完成"
	}

	title := fmt.Sprintf("%s 步骤 %d/%d %s", emoji, stepID, total, label)
	c := NewCard(title, color)

	// Progress bar (visual)
	if total > 0 {
		done := stepID
		if status == "failed" {
			done = stepID - 1
		}
		bar := progressBar(done, total)
		c.AddMarkdown(fmt.Sprintf("**进度** %s  %d/%d", bar, done, total))
	}

	c.AddDivider()
	c.AddMarkdown(fmt.Sprintf("**任务** %s", taskTitle))
	c.AddMarkdown(fmt.Sprintf("**步骤** %s", action))

	if detail != "" {
		if len(detail) > 500 {
			detail = detail[:500] + "..."
		}
		c.AddDivider()
		c.AddMarkdown(detail)
	}

	return c
}

// TaskCompletedCard builds a success card for a completed task.
func TaskCompletedCard(taskTitle, taskID, summary string) *Card {
	c := NewCard("🎉 任务完成", CardGreen)
	c.AddMarkdown(fmt.Sprintf("**%s**", taskTitle))
	if summary != "" {
		if len(summary) > 1000 {
			summary = summary[:1000] + "..."
		}
		c.AddDivider()
		c.AddMarkdown(summary)
	}
	c.AddDivider()
	c.AddNote(fmt.Sprintf("任务 ID: %s", taskID))
	return c
}

// TaskFailedCard builds a failure card for a failed task.
func TaskFailedCard(taskTitle, taskID, errMsg string) *Card {
	c := NewCard("💥 任务失败", CardRed)
	c.AddMarkdown(fmt.Sprintf("**%s**", taskTitle))
	if errMsg != "" {
		if len(errMsg) > 800 {
			errMsg = errMsg[:800] + "..."
		}
		c.AddDivider()
		c.AddMarkdown(fmt.Sprintf("```\n%s\n```", errMsg))
	}
	c.AddDivider()
	c.AddButton("重试", BtnPrimary, map[string]string{"action": "retry_task", "task_id": taskID})
	c.AddButton("取消", BtnDanger, map[string]string{"action": "cancel_task", "task_id": taskID})
	c.AddNote(fmt.Sprintf("任务 ID: %s", taskID))
	return c
}

// TaskApprovalCard builds a card requesting human approval.
func TaskApprovalCard(taskTitle, taskID, description string) *Card {
	c := NewCard("⏳ 需要审批", CardOrange)
	c.AddMarkdown(fmt.Sprintf("**%s**", taskTitle))
	if description != "" {
		c.AddDivider()
		c.AddMarkdown(description)
	}
	c.AddDivider()
	c.AddButton("批准", BtnPrimary, map[string]string{"action": "approve", "task_id": taskID})
	c.AddButton("拒绝", BtnDanger, map[string]string{"action": "deny", "task_id": taskID})
	return c
}

// progressBar generates a text-based progress bar.
func progressBar(done, total int) string {
	if total <= 0 {
		return ""
	}
	const width = 10
	filled := done * width / total
	if filled > width {
		filled = width
	}
	bar := make([]rune, width)
	for i := range bar {
		if i < filled {
			bar[i] = '█'
		} else {
			bar[i] = '░'
		}
	}
	return string(bar)
}

// ── Markdown fallback variants ──────────────────────────────
// For channels that support Markdown but not interactive cards.

// TaskStepMarkdown returns a Markdown string for step progress.
func TaskStepMarkdown(taskTitle string, stepID, total int, action, status, detail string) string {
	var emoji, label string
	switch status {
	case "failed":
		emoji = "❌"
		label = "失败"
	default:
		emoji = "✅"
		label = "完成"
	}
	md := fmt.Sprintf("**%s 步骤 %d/%d %s**\n", emoji, stepID, total, label)
	if total > 0 {
		done := stepID
		if status == "failed" {
			done = stepID - 1
		}
		md += fmt.Sprintf("进度: %s %d/%d\n", progressBar(done, total), done, total)
	}
	md += fmt.Sprintf("任务: %s\n步骤: %s\n", taskTitle, action)
	if detail != "" {
		if len(detail) > 500 {
			detail = detail[:500] + "..."
		}
		md += "\n" + detail
	}
	return md
}

// TaskCompletedMarkdown returns a Markdown string for task completion.
func TaskCompletedMarkdown(taskTitle, taskID, summary string) string {
	md := fmt.Sprintf("**🎉 任务完成**\n**%s**\n", taskTitle)
	if summary != "" {
		if len(summary) > 1000 {
			summary = summary[:1000] + "..."
		}
		md += "\n" + summary + "\n"
	}
	md += fmt.Sprintf("\n_任务 ID: %s_", taskID)
	return md
}

// TaskFailedMarkdown returns a Markdown string for task failure.
func TaskFailedMarkdown(taskTitle, taskID, errMsg string) string {
	md := fmt.Sprintf("**💥 任务失败**\n**%s**\n", taskTitle)
	if errMsg != "" {
		if len(errMsg) > 800 {
			errMsg = errMsg[:800] + "..."
		}
		md += fmt.Sprintf("\n```\n%s\n```\n", errMsg)
	}
	md += fmt.Sprintf("\n_任务 ID: %s_", taskID)
	return md
}

// ── Telegram InlineKeyboard Reply builders ──────────────────
// These return Reply with RichMessage containing ButtonComponents
// that will be converted to InlineKeyboard by telegramInlineKeyboard().

// TaskFailedReplyTG returns a Reply for task failure with retry/cancel buttons.
func TaskFailedReplyTG(taskTitle, taskID, errMsg string) Reply {
	md := TaskFailedMarkdown(taskTitle, taskID, errMsg)
	rm := NewRichMessage()
	rm.Add(NewButton("🔄 重试", "retry_task:"+taskID, "primary"))
	rm.Add(NewButton("🚫 取消", "cancel_task:"+taskID, "danger"))
	return Reply{Content: md, Format: "markdown", Rich: rm}
}

// TaskPausedReplyTG returns a Reply for task paused with resume button.
func TaskPausedReplyTG(taskID string) Reply {
	md := fmt.Sprintf("**⏸ 已暂停**\n任务 `%s` 已暂停", taskID)
	rm := NewRichMessage()
	rm.Add(NewButton("▶️ 恢复", "resume_task:"+taskID, "primary"))
	return Reply{Content: md, Format: "markdown", Rich: rm}
}

// TaskApprovalReplyTG returns a Reply for approval request with approve/deny buttons.
func TaskApprovalReplyTG(taskTitle, taskID, description string) Reply {
	md := fmt.Sprintf("**⏳ 需要审批**\n**%s**\n", taskTitle)
	if description != "" {
		md += "\n" + description + "\n"
	}
	rm := NewRichMessage()
	rm.Add(NewButton("✅ 批准", "approve:"+taskID, "primary"))
	rm.Add(NewButton("❌ 拒绝", "deny:"+taskID, "danger"))
	return Reply{Content: md, Format: "markdown", Rich: rm}
}
