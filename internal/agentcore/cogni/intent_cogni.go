package cogni

import (
	"context"
	"strings"
)

// IntentCogni detects the user's task intent from the message and returns
// the corresponding tools, skills, and memory scope needed for that intent.
//
// This is the core v2 Cogni that drives resource allocation: instead of
// injecting all 200 tools from connected MCP servers, only the 5-10 tools
// relevant to the current task are exposed to the model.
//
// Intent detection reuses the existing intentToScope logic (LocalBrain's
// IntentHint or heuristic keyword matching) and maps it to concrete resource
// requirements.
type IntentCogni struct {
	// Priority for this Cogni in decision merging (higher wins on intent conflicts)
	priority int
}

// NewIntentCogni creates an IntentCogni with the given priority.
// Recommended priority: 100 (highest, since intent detection is foundational)
func NewIntentCogni() *IntentCogni {
	return &IntentCogni{priority: 100}
}

// Analyze implements HookV2 by detecting the task intent and returning
// the matching tools, skills, and memory scope.
func (c *IntentCogni) Analyze(ctx context.Context, req CogniRequest) CogniDecision {
	intent := detectIntent(req.Message)

	switch intent {
	case "search":
		return CogniDecision{
			Intent:       &Intent{Type: "search", Confidence: 0.9},
			ToolsNeeded:  []string{"browser_search", "web_fetch"},
			SkillsNeeded: []string{"research"},
			MemoryScope: MemoryScope{
				Limit:      5,
				Categories: []string{"project"}, // 项目相关记忆（搜索上下文）
				Keywords:   []string{"搜索", "查询", "资料"},
			},
			BehaviorText: "", // Intent detection doesn't inject behavioral guidance
		}

	case "code":
		return CogniDecision{
			Intent:       &Intent{Type: "code", Confidence: 0.85},
			ToolsNeeded:  []string{"file_*", "code_*", "github_*", "gitlab_*"},
			SkillsNeeded: []string{"code"},
			MemoryScope: MemoryScope{
				Limit:      10,
				Categories: []string{"project", "identity"}, // 项目结构 + 用户偏好
				Keywords:   []string{"代码", "项目", "文件"},
			},
			BehaviorText: "",
		}

	case "chat":
		return CogniDecision{
			Intent:       &Intent{Type: "chat", Confidence: 0.8},
			ToolsNeeded:  []string{}, // 闲聊不需要工具
			SkillsNeeded: []string{}, // 闲聊不需要技能
			MemoryScope: MemoryScope{
				Limit:      15,
				Categories: []string{"conversation", "identity"}, // 对话历史 + 用户身份
				Keywords:   []string{"聊天", "对话", "情感"},
			},
			BehaviorText: "", // Emotion Cogni will handle tone adjustment
		}

	case "browser":
		return CogniDecision{
			Intent:       &Intent{Type: "browser", Confidence: 0.85},
			ToolsNeeded:  []string{"browser_*"},
			SkillsNeeded: []string{"browser"},
			MemoryScope: MemoryScope{
				Limit:      8,
				Categories: []string{"project"},
				Keywords:   []string{"浏览器", "网页", "点击"},
			},
			BehaviorText: "",
		}

	case "file":
		return CogniDecision{
			Intent:       &Intent{Type: "file", Confidence: 0.85},
			ToolsNeeded:  []string{"file_*"},
			SkillsNeeded: []string{"file"},
			MemoryScope: MemoryScope{
				Limit:      8,
				Categories: []string{"project"},
				Keywords:   []string{"文件", "目录", "路径"},
			},
			BehaviorText: "",
		}

	case "complex":
		fallthrough
	default:
		// Complex or unknown intent → fallback to broader resource allocation
		// Don't restrict tools/skills, let other Cognis decide
		return CogniDecision{
			Intent:       &Intent{Type: "complex", Confidence: 0.5},
			ToolsNeeded:  nil, // nil means no restriction (all tools available)
			SkillsNeeded: nil,
			MemoryScope: MemoryScope{
				Limit:      20, // Default limit
				Categories: []string{}, // No category filter
			},
			BehaviorText: "",
		}
	}
}

// Priority returns this Cogni's priority in decision merging.
// IntentCogni has the highest priority (100) since intent detection is foundational.
func (c *IntentCogni) Priority() int {
	return c.priority
}

// detectIntent performs heuristic intent detection based on message keywords.
// This is a simple implementation; production should use LocalBrain's IntentHint
// or a more sophisticated classifier.
//
// Priority: more specific patterns first (code > search) to avoid false positives.
func detectIntent(message string) string {
	lower := strings.ToLower(message)

	// Code intent (check before search to avoid "审查" being caught by "查")
	if containsAny(lower, []string{"代码", "code", "pr", "pull request", "commit", "git", "github", "gitlab", "仓库", "repository", "repo", "审查", "review"}) {
		return "code"
	}

	// Browser intent
	if containsAny(lower, []string{"浏览器", "网页", "点击", "browser", "navigate", "click", "fill", "screenshot", "打开网页"}) {
		return "browser"
	}

	// Search intent (after code, to avoid overlap)
	if containsAny(lower, []string{"搜索", "search", "find", "lookup", "查找", "查询"}) {
		// But not if it's code-related search
		if !containsAny(lower, []string{"代码", "文件", "项目"}) {
			return "search"
		}
	}

	// Chat intent (casual conversation, emotional topics)
	if containsAny(lower, []string{"聊", "心情", "感觉", "情绪", "陪", "能不能", "可以吗", "吗", "呢", "啊", "哦"}) {
		return "chat"
	}

	// Complex: multi-step tasks with broad scope
	if containsAny(lower, []string{"完整", "整个", "全部", "所有", "一系列", "multiple", "entire", "whole", "complete"}) {
		return "complex"
	}

	// File operations (subset of code, but distinct)
	if containsAny(lower, []string{"文件", "file", "目录", "directory", "路径", "path"}) {
		return "file"
	}

	// Default to complex for unknown
	return "complex"
}

// containsAny returns true if s contains any of the keywords.
func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
