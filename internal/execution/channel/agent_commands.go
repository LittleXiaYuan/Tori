package channel

import (
	"fmt"
	"strings"
)

// AgentCommands provides universal slash command handlers for all IM channels.
// Callbacks are injected to avoid circular dependencies with gateway/planner.
type AgentCommands struct {
	// GetThinkingLevel returns the current thinking level for a user.
	GetThinkingLevel func(channelType, userID string) string

	// SetThinkingLevel sets thinking level for a user. Returns confirmation message.
	SetThinkingLevel func(channelType, userID, level string) string

	// GetStatus returns formatted agent status (model, skills, memory).
	GetStatus func() string

	// ParseMission parses NL text and returns a formatted preview string.
	ParseMission func(description string) (string, error)

	// CreateMission creates a mission from NL description. Returns confirmation.
	CreateMission func(description string) (string, error)
}

// Handler returns a CommandHandler for agent commands.
// Recognized: /think, /status, /mission, /help, /start, /skills
func (ac *AgentCommands) Handler() CommandHandler {
	return func(msg Message, command, args string) (Reply, bool) {
		switch command {
		case "/think":
			return ac.handleThink(msg, args), true
		case "/status":
			return ac.handleStatus(), true
		case "/mission":
			return ac.handleMission(msg, args), true
		case "/help", "/start":
			return ac.handleHelp(msg), true
		case "/skills":
			return ac.handleSkills(), true
		}
		return Reply{}, false
	}
}

func (ac *AgentCommands) handleThink(msg Message, args string) Reply {
	args = strings.TrimSpace(strings.ToLower(args))

	// No args → show current level
	if args == "" {
		if ac.GetThinkingLevel == nil {
			return Reply{Content: "当前思维模式: auto", Format: "text"}
		}
		level := ac.GetThinkingLevel(msg.ChannelType, msg.UserID)
		labels := map[string]string{
			"none": " 快速 (none) — 极速响应，跳过深度推理",
			"auto": " 自动 (auto) — 智能路由，按需分配模型",
			"deep": " 深度 (deep) — 专家模型，深度推理和分析",
		}
		label := labels[level]
		if label == "" {
			label = level
		}
		return Reply{
			Content: fmt.Sprintf("当前思维模式: %s\n\n切换: /think none | auto | deep", label),
			Format:  "text",
		}
	}

	// Validate and set
	valid := map[string]bool{"none": true, "auto": true, "deep": true}
	// Accept Chinese aliases
	aliases := map[string]string{"快速": "none", "自动": "auto", "深度": "deep", "fast": "none"}
	if mapped, ok := aliases[args]; ok {
		args = mapped
	}
	if !valid[args] {
		return Reply{
			Content: "无效的思维模式。可选: none (快速) | auto (自动) | deep (深度)",
			Format:  "text",
		}
	}

	if ac.SetThinkingLevel == nil {
		return Reply{Content: "思维模式控制未启用", Format: "text"}
	}

	result := ac.SetThinkingLevel(msg.ChannelType, msg.UserID, args)
	return Reply{Content: result, Format: "text"}
}

func (ac *AgentCommands) handleStatus() Reply {
	if ac.GetStatus == nil {
		return Reply{Content: "状态查询未启用", Format: "text"}
	}
	return Reply{Content: ac.GetStatus(), Format: "markdown"}
}

func (ac *AgentCommands) handleMission(msg Message, args string) Reply {
	args = strings.TrimSpace(args)
	if args == "" {
		return Reply{
			Content: "用法: /mission <任务描述>\n\n" +
				"示例:\n" +
				"  /mission 每天早上9点提醒我看日报\n" +
				"  /mission 当有新邮件时自动摘要并推送\n" +
				"  /mission 搜索今天的AI新闻并整理",
			Format: "text",
		}
	}

	if ac.CreateMission != nil {
		result, err := ac.CreateMission(args)
		if err != nil {
			return Reply{Content: fmt.Sprintf("创建失败: %s", err), Format: "text"}
		}
		return Reply{Content: result, Format: "markdown"}
	}

	// Fallback: parse only
	if ac.ParseMission != nil {
		preview, err := ac.ParseMission(args)
		if err != nil {
			return Reply{Content: fmt.Sprintf("解析失败: %s", err), Format: "text"}
		}
		return Reply{Content: preview, Format: "markdown"}
	}

	return Reply{Content: "编排功能未启用", Format: "text"}
}

func (ac *AgentCommands) handleHelp(msg Message) Reply {
	help := " *云鸢 Agent 命令*\n\n" +
		"*对话*\n" +
		"  直接发消息即可对话\n\n" +
		"*思维控制*\n" +
		"  /think — 查看当前思维模式\n" +
		"  /think deep — 切换深度推理\n" +
		"  /think auto — 切换自动路由\n" +
		"  /think none — 切换快速响应\n\n" +
		"*任务编排*\n" +
		"  /mission <描述> — 自然语言创建任务\n" +
		"  /status — 查看 Agent 状态\n\n" +
		"*贴图*\n" +
		"  /sticker — 查看贴图库\n" +
		"  /add <情绪> — 添加贴图\n" +
		"  /add-all — 批量添加贴图\n\n" +
		"  /help — 显示此帮助"

	return Reply{Content: help, Format: "markdown"}
}

func (ac *AgentCommands) handleSkills() Reply {
	if ac.GetStatus != nil {
		return Reply{Content: ac.GetStatus(), Format: "markdown"}
	}
	return Reply{
		Content: "🔧 *可用技能*\n\n" +
			"• web_search — 搜索\n" +
			"• code_execute — 代码执行\n" +
			"• file_search — 文件搜索\n" +
			"• doc_generate — 文档生成\n\n" +
			"使用 /status 查看完整状态",
		Format: "markdown",
	}
}
