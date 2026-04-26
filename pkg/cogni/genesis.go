package cogni

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// LLMFunc is the host-provided LLM call primitive used by Genesis.
type LLMFunc func(ctx context.Context, system, user string) (string, error)

// Genesis generates a Cogni Declaration from a natural-language description.
// It uses the host's LLM to produce a structured JSON declaration, then
// validates it before returning.
type Genesis struct {
	llm LLMFunc
}

func NewGenesis(llm LLMFunc) *Genesis {
	return &Genesis{llm: llm}
}

const genesisSystemPrompt = `你是一个 Cogni（智体）配置生成器。用户会用自然语言描述他们想要的 AI 智体，你需要生成一个完整的 Cogni 声明 JSON。

## 输出格式
只输出一个合法的 JSON 对象，不要输出其他内容。JSON 必须符合以下结构：

{
  "id": "kebab-case 唯一标识",
  "display_name": "中文显示名",
  "description": "简短描述",
  "activation": {
    "keywords": ["触发关键词"],
    "regex": ["正则表达式（可选）"],
    "min_score": 0.3,
    "always_on": false
  },
  "context": {
    "static": "注入到 system prompt 的静态文本，描述智体的角色和行为规范",
    "memory_query": "用于记忆召回的查询模板，{message} 会被替换为用户消息（可选）"
  },
  "surface": {
    "exclude": ["要排除的危险工具名（可选）"],
    "include": ["要包含的工具名（可选）"]
  },
  "memory": {
    "namespace": "记忆命名空间（与 id 相同）"
  },
  "workflows": [],
  "experience": {
    "enabled": true,
    "auto_record": true,
    "require_review": true,
    "half_life_days": 90,
    "max_facts": 500
  },
  "checks": [
    {"message": "应该触发的示例消息", "expect_active": true},
    {"message": "不应触发的示例消息", "expect_active": false}
  ]
}

## 规则
1. id 必须是 kebab-case，简短有意义
2. keywords 要覆盖中英文常见表达
3. context.static 要详细描述角色、能力、行为准则
4. 如果用户描述涉及危险操作（如代码执行、文件修改），在 surface.exclude 中排除相关工具
5. checks 至少包含 2 条正例和 1 条反例
6. 如果用户描述了多步工作流，在 workflows 中定义`

// Generate produces a Declaration from a natural-language description.
func (g *Genesis) Generate(ctx context.Context, description string) (*Declaration, error) {
	if g.llm == nil {
		return nil, fmt.Errorf("genesis: LLM not configured")
	}

	raw, err := g.llm(ctx, genesisSystemPrompt, description)
	if err != nil {
		return nil, fmt.Errorf("genesis: LLM call failed: %w", err)
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, fmt.Errorf("genesis: LLM response contains no valid JSON")
	}

	var d Declaration
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		return nil, fmt.Errorf("genesis: parse LLM output: %w", err)
	}

	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("genesis: validation failed: %w", err)
	}

	return &d, nil
}

// extractJSON finds the first JSON object in the LLM response, handling
// common patterns like markdown code blocks.
func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)

	// Strip markdown code fences
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		start, end := -1, -1
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if start < 0 && strings.HasPrefix(trimmed, "```") {
				start = i + 1
			} else if start >= 0 && strings.HasPrefix(trimmed, "```") {
				end = i
				break
			}
		}
		if start >= 0 && end > start {
			raw = strings.Join(lines[start:end], "\n")
		} else if start >= 0 {
			raw = strings.Join(lines[start:], "\n")
		}
	}

	raw = strings.TrimSpace(raw)

	// Find first { ... } block
	start := strings.Index(raw, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}
