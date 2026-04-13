package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/pkg/skills"
)

// ── useSkillTool ──
// Wraps an installed skill for invocation via the task runner.

type useSkillTool struct {
	installer *skillmarket.Installer
}

func (t *useSkillTool) Name() string        { return "use_skill" }
func (t *useSkillTool) Description() string { return "加载已安装技能列表并返回描述" }
func (t *useSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"slug": map[string]any{
				"type":        "string",
				"description": "技能标识符",
			},
		},
		"required": []string{"slug"},
	}
}

func (t *useSkillTool) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	slug, _ := args["slug"].(string)
	if slug == "" {
		return "", fmt.Errorf("slug is required")
	}

	installed := t.installer.Installed()
	for _, info := range installed {
		if info.Slug == slug {
			return fmt.Sprintf("已安装技能: %s\n描述: %s\n版本: %s\n状态: 已启用=%v",
				info.Slug, info.Description, info.Version, info.Enabled), nil
		}
	}
	return "", fmt.Errorf("skill %q not found in installed list", slug)
}

// ── searchSkillsTool ──
// Searches the ClawHub marketplace for available skills.

type searchSkillsTool struct {
	provider *skillmarket.ClawHubProvider
}

func (t *searchSkillsTool) Name() string        { return "search_skills" }
func (t *searchSkillsTool) Description() string { return "在技能市场中搜索可用技能" }
func (t *searchSkillsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "搜索关键词",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "返回结果数量上限",
			},
		},
		"required": []string{"query"},
	}
}

func (t *searchSkillsTool) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	results, err := t.provider.Search(query, limit)
	if err != nil {
		return "", fmt.Errorf("搜索技能失败: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("未找到与 %q 相关的技能", query), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("搜索 %q 结果 (%d 个):\n\n", query, len(results)))
	for i, r := range results {
		b.WriteString(fmt.Sprintf("%d. %s (%s)\n   %s\n", i+1, r.Name, r.Slug, r.Description))
		if r.Version != "" {
			b.WriteString(fmt.Sprintf("   版本: %s\n", r.Version))
		}
	}
	return b.String(), nil
}

// ── installSkillTool ──
// Installs a skill from ClawHub by slug.

type installSkillTool struct {
	installer *skillmarket.Installer
}

func (t *installSkillTool) Name() string        { return "install_skill" }
func (t *installSkillTool) Description() string { return "安装指定技能到本地。支持 ClawHub 技能(slug)和 GitHub 仓库(owner/repo 或 owner/repo/path)" }
func (t *installSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"slug": map[string]any{
				"type":        "string",
				"description": "技能标识符: ClawHub slug 如 'my-skill'，或 GitHub 路径如 'owner/repo' / 'owner/repo/path/to/skill'",
			},
		},
		"required": []string{"slug"},
	}
}

func (t *installSkillTool) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	slug, _ := args["slug"].(string)
	if slug == "" {
		return "", fmt.Errorf("slug is required")
	}

	report, err := t.installer.Install(ctx, slug)
	if err != nil {
		return "", fmt.Errorf("安装技能 %q 失败: %w", slug, err)
	}

	data, _ := json.MarshalIndent(report, "", "  ")
	return fmt.Sprintf("技能 %q 安装完成。\n审计报告:\n%s", slug, string(data)), nil
}

// ── marketplaceSkill ──
// An LLM-backed skill created from a SKILL.md installed via SkillHub.
// The SKILL.md content is used as a system prompt so the LLM follows its instructions.
// This makes marketplace skills immediately executable without writing Python/Node scripts.

type marketplaceSkill struct {
	slug        string
	name        string
	description string
	content     string // SKILL.md body
	llmCall     func(ctx context.Context, system, user string) (string, error)
}

func (s *marketplaceSkill) Name() string        { return s.name }
func (s *marketplaceSkill) Description() string { return s.description }

func (s *marketplaceSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type":        "string",
				"description": "用户输入或任务描述",
			},
		},
		"required": []string{"input"},
	}
}

func (s *marketplaceSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	input, _ := args["input"].(string)
	if input == "" {
		return "", fmt.Errorf("input is required")
	}

	systemPrompt := fmt.Sprintf("你是一个专用技能模块。请严格按照以下 SKILL.md 的说明执行任务。\n\n---\n%s\n---\n\n用户的输入就是你要处理的任务。只输出执行结果。", s.content)

	reply, err := s.llmCall(ctx, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("marketplace skill %q: %w", s.name, err)
	}
	return reply, nil
}
