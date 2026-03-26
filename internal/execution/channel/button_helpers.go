package channel

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ContentWithButtonFallback appends plain-text hints for Button components so channels
// without native inline keyboards still expose the same choices.
func ContentWithButtonFallback(reply Reply) string {
	base := reply.Content
	if reply.Rich == nil {
		return base
	}
	var sb strings.Builder
	sb.WriteString(base)
	for _, c := range reply.Rich.Components {
		b, ok := c.(*ButtonComponent)
		if !ok || b.URL != "" {
			continue
		}
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteString("\n")
		}
		val := b.Value
		if val == "" {
			val = b.Label
		}
		sb.WriteString(fmt.Sprintf("[%s] 请直接回复：%s", b.Label, val))
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// ParseNumberChoice parses a user message like "1" or "选 2" against n options (1..n).
// Returns 1-based index, or 0 if not matched.
func ParseNumberChoice(text string, n int) int {
	text = strings.TrimSpace(text)
	if n <= 0 {
		return 0
	}
	if i, err := strconv.Atoi(text); err == nil && i >= 1 && i <= n {
		return i
	}
	var digits strings.Builder
	for _, r := range text {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if i, err := strconv.Atoi(digits.String()); err == nil && i >= 1 && i <= n {
		return i
	}
	return 0
}

// TruncateRunes shortens s to max runes (for Telegram callback_data limits, etc.).
func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}
