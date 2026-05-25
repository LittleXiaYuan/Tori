package planner

// prompt.go — System prompt generation and reply cleaning utilities.
// Handles building the agent's system prompt with L1/L2/L3 skill tiers,
// cleaning LLM output (tool call JSON, think blocks, ACT tags),
// and related text manipulation helpers.

import (
	"embed"
	"strings"
)

//go:embed prompts/*/*.tmpl
var promptFiles embed.FS

// InvalidatePromptCache forces rebuild of the cached system prompt on next call.
func (p *Planner) InvalidatePromptCache() {
	p.ensurePromptRuntime().InvalidatePromptCache()
}

func (p *Planner) buildSystemPrompt() string {
	return p.ensurePromptRuntime().BuildSystemPrompt(p.registry)
}

func (p *Planner) buildSubagentSystemPrompt() string {
	return p.ensurePromptRuntime().BuildSubagentSystemPrompt(p.registry)
}

// cleanReply removes internal artifacts from LLM output before presenting to users.
func (p *Planner) cleanReply(text string) string {
	// Remove JSON skill call blocks (may appear multiple times)
	for _, marker := range []string{`"tool_calls"`, `"skill_calls"`} {
		for {
			idx := strings.Index(text, marker)
			if idx < 0 {
				break
			}
			start := strings.LastIndex(text[:idx], "{")
			if start < 0 {
				break
			}
			end := findClosingBrace(text, start)
			if end >= 0 {
				text = strings.TrimSpace(text[:start] + text[end+1:])
			} else {
				break
			}
		}
	}
	// Remove ```json blocks
	for {
		s := strings.Index(text, "```json")
		if s < 0 {
			s = strings.Index(text, "```JSON")
		}
		if s < 0 {
			break
		}
		e := strings.Index(text[s+7:], "```")
		if e < 0 {
			break
		}
		text = strings.TrimSpace(text[:s] + text[s+7+e+3:])
	}
	// Remove <think>...</think>
	for {
		s := strings.Index(text, "<think>")
		if s < 0 {
			break
		}
		e := strings.Index(text[s:], "</think>")
		if e < 0 {
			text = strings.TrimSpace(text[:s])
			break
		}
		text = strings.TrimSpace(text[:s] + text[s+e+8:])
	}
	// Remove <|ACT {...}|> inline emotion/action tags
	for {
		s := strings.Index(text, "<|ACT ")
		if s < 0 {
			break
		}
		e := strings.Index(text[s:], "|>")
		if e < 0 {
			break
		}
		// Remove the tag and any trailing newline on the same line
		endPos := s + e + 2
		if endPos < len(text) && text[endPos] == '\n' {
			endPos++
		}
		text = text[:s] + text[endPos:]
	}
	text = strings.TrimSpace(text)
	text = stripInlineReasoning(text)
	text = cleanTrailingCallDescription(text)
	return strings.TrimSpace(text)
}

// stripInlineReasoning removes leading "thinking out loud" paragraphs that some models
// emit before the actual reply (e.g., "用户发送了xxx...我应该...").
// Detects consecutive analysis paragraphs and strips them, keeping the user-facing content.
func stripInlineReasoning(text string) string {
	reasoningPrefixes := []string{
		"用户",
		"这是一个", "这是关于", "这个问题",
		"我需要", "我应该", "我来", "让我", "我将",
		"根据规范", "根据要求", "根据上下文",
		"好的，", "好的,", "好的。",
		"分析：", "分析:",
		"看起来", "首先",
	}

	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) <= 1 {
		return text
	}

	firstRealIdx := 0
	for i, para := range paragraphs {
		trimmed := strings.TrimSpace(para)
		if trimmed == "" {
			continue
		}
		isReasoning := false
		for _, prefix := range reasoningPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				isReasoning = true
				break
			}
		}
		if !isReasoning {
			firstRealIdx = i
			break
		}
		firstRealIdx = i + 1
	}

	if firstRealIdx == 0 || firstRealIdx >= len(paragraphs) {
		return text
	}

	result := strings.Join(paragraphs[firstRealIdx:], "\n\n")
	if strings.TrimSpace(result) == "" {
		return text
	}
	return strings.TrimSpace(result)
}

// cleanTrailingCallDescription removes trailing sentences where the LLM describes
// calling a tool but the actual JSON was already stripped, leaving orphaned text like
// "让我先调用use_skill来加载Chirp的详细说明：" at the end.
func cleanTrailingCallDescription(text string) string {
	// Patterns that indicate "I'm going to call a tool" — orphaned at end of text
	suffixes := []string{
		"让我", "我来调用", "我先调用", "让我先", "让我尝试", "我将调用", "我会调用",
		"Let me call", "I'll invoke", "Let me try",
	}
	trimmed := strings.TrimSpace(text)
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return text
	}
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	// Only strip if the last line looks like an orphaned tool call description
	for _, suffix := range suffixes {
		if strings.Contains(lastLine, suffix) && (strings.HasSuffix(lastLine, "：") || strings.HasSuffix(lastLine, ":")) {
			return strings.TrimSpace(strings.Join(lines[:len(lines)-1], "\n"))
		}
	}
	return text
}

// truncate shortens a string to maxLen runes (not bytes), appending "..." if truncated.
// Uses rune-based counting to safely handle CJK/multi-byte characters.
func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}

// findClosingBrace finds the matching closing brace for an opening brace at position start.
func findClosingBrace(s string, start int) int {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractNextMoves splits reply text at the ---NEXT--- marker.
// Returns the cleaned reply and a list of suggestion strings.
func extractNextMoves(text string) (string, []string) {
	markers := []string{"---NEXT---", "--- NEXT ---", "---NEXT MOVES---"}
	idx := -1
	for _, m := range markers {
		if i := strings.Index(text, m); i >= 0 {
			if idx < 0 || i < idx {
				idx = i
			}
		}
	}
	if idx < 0 {
		return text, nil
	}

	reply := strings.TrimSpace(text[:idx])
	tail := text[idx:]

	// Skip past the marker line
	lines := strings.Split(tail, "\n")
	var suggestions []string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" || line == "```" {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		// Strip numbered prefixes like "1. " "2. "
		if len(line) > 2 && line[1] == '.' && line[0] >= '1' && line[0] <= '9' {
			line = strings.TrimSpace(line[2:])
		}
		if line != "" {
			suggestions = append(suggestions, line)
		}
	}
	return reply, suggestions
}
