package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

// ── generateSkillTool ──
// Generates new skills as file-based packages in data/skills/<slug>/.
// Each generated skill is a standalone directory with SKILL.md, meta.json,
// and optional scripts/templates, ready for extraction and submission.

type generateSkillTool struct {
	llmCall  func(ctx context.Context, system, user string) (string, error)
	skillDir string // e.g. data/skills
	onReload func() // trigger SkillFileLoader reload + prompt cache invalidation
}

func (t *generateSkillTool) Name() string { return "generate_skill" }
func (t *generateSkillTool) Description() string {
	return "根据自然语言描述生成新技能（含 SKILL.md、meta.json、可选脚本），保存到 data/skills/ 并自动注册。用户说\"创建/生成一个能做XX的技能\"时调用此工具。"
}
func (t *generateSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type":        "string",
				"description": "技能功能的自然语言描述，例如：\"一个能将Markdown转换为思维导图的技能\"",
			},
		},
		"required": []string{"description"},
	}
}

func (t *generateSkillTool) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	desc, _ := args["description"].(string)
	if desc == "" {
		return "", fmt.Errorf("description is required")
	}

	slog.Info("generate_skill: calling LLM", "desc", truncateStr(desc, 80))
	reply, err := t.llmCall(ctx, generateSkillSystemPrompt, fmt.Sprintf("请根据以下描述生成完整技能包：\n%s", desc))
	if err != nil {
		return "", fmt.Errorf("LLM 生成失败: %w", err)
	}
	slog.Info("generate_skill: LLM replied", "len", len(reply), "head", truncateStr(reply, 200))

	pkg, err := parseGeneratedSkillPkg(reply)
	if err != nil {
		slog.Error("generate_skill: parse failed", "err", err, "reply_len", len(reply))
		return "", fmt.Errorf("解析技能包失败: %w", err)
	}
	slog.Info("generate_skill: parsed", "slug", pkg.Slug, "name", pkg.Name, "files", len(pkg.Files))

	pkgDir := filepath.Join(t.skillDir, pkg.Slug)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	var writtenFiles []string
	for _, f := range pkg.Files {
		fPath := filepath.Join(pkgDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(fPath), 0755); err != nil {
			return "", fmt.Errorf("创建子目录失败: %w", err)
		}
		if err := os.WriteFile(fPath, []byte(f.Content), 0644); err != nil {
			return "", fmt.Errorf("写入 %s 失败: %w", f.Path, err)
		}
		writtenFiles = append(writtenFiles, f.Path)
	}
	slog.Info("generate_skill: files written", "dir", pkgDir, "count", len(writtenFiles))

	if t.onReload != nil {
		t.onReload()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("技能 \"%s\" 已生成并注册\n\n", pkg.Name))
	sb.WriteString(fmt.Sprintf("位置: %s\n", pkgDir))
	sb.WriteString(fmt.Sprintf("描述: %s\n", pkg.Desc))
	sb.WriteString("文件列表:\n")
	for _, f := range writtenFiles {
		sb.WriteString(fmt.Sprintf("   - %s\n", f))
	}
	sb.WriteString("\n技能已自动加载到系统中，可以立即使用。")
	return sb.String(), nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

type generatedFile struct {
	Path    string
	Content string
}

type generatedPkg struct {
	Slug  string
	Name  string
	Desc  string
	Files []generatedFile
}

func parseGeneratedSkillPkg(reply string) (*generatedPkg, error) {
	reply = strings.ReplaceAll(reply, "\r\n", "\n")
	pkg := &generatedPkg{}

	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "SKILL_SLUG:"):
			pkg.Slug = strings.TrimSpace(line[len("SKILL_SLUG:"):])
		case strings.HasPrefix(line, "SKILL_NAME:"):
			pkg.Name = strings.TrimSpace(line[len("SKILL_NAME:"):])
		case strings.HasPrefix(line, "SKILL_DESC:"):
			pkg.Desc = strings.TrimSpace(line[len("SKILL_DESC:"):])
		}
	}

	remaining := reply
	for {
		fileStart := strings.Index(remaining, "---FILE:")
		if fileStart < 0 {
			break
		}
		afterMarker := remaining[fileStart+len("---FILE:"):]
		nlPos := strings.Index(afterMarker, "\n")
		if nlPos < 0 {
			break
		}
		filePath := strings.TrimSpace(afterMarker[:nlPos])
		filePath = strings.TrimSuffix(filePath, "---")
		filePath = strings.TrimSpace(filePath)

		contentStart := fileStart + len("---FILE:") + nlPos + 1
		fileEnd := strings.Index(remaining[contentStart:], "---END_FILE---")
		if fileEnd < 0 {
			break
		}
		content := strings.TrimSpace(remaining[contentStart : contentStart+fileEnd])
		// Strip markdown code fences the LLM might wrap content in
		if strings.HasPrefix(content, "```") {
			if idx := strings.Index(content, "\n"); idx >= 0 {
				content = content[idx+1:]
			}
			if strings.HasSuffix(content, "```") {
				content = content[:len(content)-3]
			}
			content = strings.TrimSpace(content)
		}
		if filePath != "" && content != "" {
			pkg.Files = append(pkg.Files, generatedFile{Path: filePath, Content: content})
		}
		remaining = remaining[contentStart+fileEnd+len("---END_FILE---"):]
	}

	if pkg.Slug == "" || pkg.Name == "" || len(pkg.Files) == 0 {
		return nil, fmt.Errorf("技能包不完整: slug=%q name=%q files=%d\nraw_head=%s",
			pkg.Slug, pkg.Name, len(pkg.Files), truncateStr(reply, 500))
	}
	if pkg.Desc == "" {
		pkg.Desc = pkg.Name
	}
	return pkg, nil
}

const generateSkillSystemPrompt = `你是技能生成器。根据用户描述生成完整技能包，包含以下文件：
1. SKILL.md — Agent 执行指令
2. meta.json — 元数据
3. README.md — 说明文档
4. scripts/main.py — 可独立运行的 Python 脚本
5. scripts/requirements.txt — Python 依赖

输出格式（严格遵循，不要输出其他内容）：
SKILL_SLUG: <slug>
SKILL_NAME: <名称>
SKILL_DESC: <一句话>
---FILE: SKILL.md---
# <技能名>
<功能说明>
## 执行方式
运行 scripts/main.py，传入参数。
## 步骤
1. ...
2. ...
## 输入参数
- query: ...
## 输出
...
---END_FILE---
---FILE: meta.json---
{"name":"<slug>","description":"<描述>","version":"1.0.0","author":"yunque-agent","tags":[],"parameters":{"query":{"type":"string","description":"...","required":true}}}
---END_FILE---
---FILE: README.md---
# <技能名>
## 功能
<详细功能描述>
## 快速使用
pip install -r scripts/requirements.txt
python scripts/main.py --query "xxx"
## 参数
| 参数 | 类型 | 说明 |
|------|------|------|
| ... | ... | ... |
## 输出示例
<示例>
## 作者
yunque-agent (自动生成)
---END_FILE---
---FILE: scripts/main.py---
<完整可运行的 Python 脚本，包含 argparse 参数解析、核心逻辑、错误处理、结果输出>
---END_FILE---
---FILE: scripts/requirements.txt---
<依赖列表，每行一个包名>
---END_FILE---

要求：
- scripts/main.py 必须可独立运行（python scripts/main.py --query "xxx"）
- 脚本要有完整的错误处理和中文输出
- 结果以 JSON 或 Markdown 格式输出到 stdout`
