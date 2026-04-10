package localbrain

import (
	"encoding/json"
	"strings"
)

// extractJSON 从可能包含额外文本的 LLM 输出中提取 JSON。
func extractJSON(raw string, target interface{}) error {
	// 先尝试直接解析
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), target); err == nil {
		return nil
	}

	// 尝试提取 ```json ... ``` 块
	if idx := strings.Index(raw, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(raw[start:], "```")
		if end > 0 {
			return json.Unmarshal([]byte(strings.TrimSpace(raw[start:start+end])), target)
		}
	}

	// 尝试找到第一个 { 和最后一个 }
	start := strings.IndexByte(raw, '{')
	end := strings.LastIndexByte(raw, '}')
	if start >= 0 && end > start {
		return json.Unmarshal([]byte(raw[start:end+1]), target)
	}

	return json.Unmarshal([]byte(raw), target) // 最终返回原始错误
}

// truncate 截断字符串到指定长度。
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
