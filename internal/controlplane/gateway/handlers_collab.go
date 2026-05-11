package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
)

func feishuMessageText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &content); err == nil && strings.TrimSpace(content.Text) != "" {
		return strings.TrimSpace(content.Text)
	}
	return raw
}

func parseCollabCommand(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	fields := strings.Fields(text)
	for i, field := range fields {
		candidate := strings.TrimFunc(field, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSpace(r)
		})
		if !strings.HasPrefix(candidate, "yq_") {
			continue
		}
		remaining := append([]string{}, fields[:i]...)
		remaining = append(remaining, fields[i+1:]...)
		content := strings.TrimSpace(strings.Join(remaining, " "))
		content = strings.TrimSpace(strings.TrimPrefix(content, "/yq"))
		content = strings.TrimSpace(strings.TrimPrefix(content, "yq"))
		return candidate, content, true
	}
	return "", "", false
}

func (g *Gateway) handleCollabInbound(ctx context.Context, code, content, channelType, channelID string) string {
	if g.notifier == nil {
		return "协作同步尚未初始化。"
	}
	binding, ok := g.notifier.GetShareBinding(code)
	if !ok || binding == nil {
		return fmt.Sprintf("未找到协作码 %s。请确认从云雀同步卡片中复制了完整协作码。", code)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Sprintf("已找到云雀会话：%s。请在协作码后输入要继续讨论的内容，例如：/yq %s 帮我继续修改表格。", binding.Title, code)
	}
	if g.convStore == nil || g.planner == nil {
		return "云雀 Chat 会话服务尚未就绪。"
	}
	tenantID := "default"
	g.convStore.GetOrCreate(binding.SessionID, tenantID)
	history := g.convStore.Get(binding.SessionID)
	userMsg := llm.Message{Role: "user", Content: fmt.Sprintf("[来自%s协作回复] %s", channelType, content)}
	msgs := append([]llm.Message{}, history...)
	msgs = append(msgs, userMsg)
	g.convStore.Append(binding.SessionID, userMsg)
	result, err := g.planner.Run(ctx, planner.PlanRequest{
		Messages:    msgs,
		TenantID:    tenantID,
		ChannelType: channelType,
	})
	if err != nil {
		return fmt.Sprintf("云雀处理协作回复失败：%s", err.Error())
	}
	g.convStore.Append(binding.SessionID, llm.Message{Role: "assistant", Content: result.Reply})
	g.notifier.TouchShareBinding(code)
	return fmt.Sprintf("已同步到云雀会话：%s\n\n%s", binding.Title, truncateCollabReply(result.Reply, 1800))
}

func truncateCollabReply(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "\n\n...已截断，可点击云雀任务链接查看完整回复。"
}
