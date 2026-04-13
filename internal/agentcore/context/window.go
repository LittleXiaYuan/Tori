package context

import (
	"strings"
	"unicode/utf8"
)

// Message is a minimal chat message for context windowing.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// WindowConfig controls how context is trimmed.
type WindowConfig struct {
	MaxTokens        int // total token budget for context
	SystemReserve    int // tokens reserved for system prompt + persona + skills
	ReplyReserve     int // tokens reserved for model reply
	MaxMessages      int // hard cap on message count
	PreserveFirst    int // always keep the first N messages (e.g., system)
	PreserveLast     int // always keep the last N messages (e.g., current turn)
}

// DefaultConfig returns sensible defaults for context windowing.
func DefaultConfig() WindowConfig {
	return WindowConfig{
		MaxTokens:     128000,
		SystemReserve: 4096,
		ReplyReserve:  4096,
		MaxMessages:   100,
		PreserveFirst: 1,
		PreserveLast:  2,
	}
}

// ConfigForWindow returns a WindowConfig scaled to the model's context window (in K tokens).
// Falls back to DefaultConfig for unknown or zero values.
func ConfigForWindow(contextWindowK int) WindowConfig {
	cfg := DefaultConfig()
	if contextWindowK > 0 {
		cfg.MaxTokens = contextWindowK * 1024
		cfg.ReplyReserve = min(8192, contextWindowK*1024/16)
	}
	return cfg
}

// TrimResult holds the outcome of context trimming.
type TrimResult struct {
	Messages       []Message
	TotalTokens    int
	DroppedCount   int
	OriginalCount  int
}

// TrimToFit trims a message history to fit within the token budget.
// It preserves the first PreserveFirst and last PreserveLast messages,
// then drops oldest middle messages until the budget is met.
func TrimToFit(messages []Message, cfg WindowConfig) TrimResult {
	if cfg.MaxTokens <= 0 {
		cfg = DefaultConfig()
	}

	original := len(messages)
	budget := cfg.MaxTokens - cfg.SystemReserve - cfg.ReplyReserve
	if budget < 0 {
		budget = 0
	}

	// Hard cap on message count
	if cfg.MaxMessages > 0 && len(messages) > cfg.MaxMessages {
		messages = messages[len(messages)-cfg.MaxMessages:]
	}

	// Calculate token costs
	costs := make([]int, len(messages))
	total := 0
	for i, m := range messages {
		c := EstimateTokens(m.Content)
		costs[i] = c
		total += c
	}

	if total <= budget {
		return TrimResult{Messages: messages, TotalTokens: total, OriginalCount: original}
	}

	// Mark which messages to keep
	keep := make([]bool, len(messages))
	for i := 0; i < cfg.PreserveFirst && i < len(messages); i++ {
		keep[i] = true
	}
	for i := len(messages) - cfg.PreserveLast; i < len(messages); i++ {
		if i >= 0 {
			keep[i] = true
		}
	}

	// Drop middle messages from oldest until budget met
	for total > budget {
		dropped := false
		for i := cfg.PreserveFirst; i < len(messages)-cfg.PreserveLast; i++ {
			if keep[i] {
				continue
			}
			total -= costs[i]
			costs[i] = 0
			keep[i] = false
			dropped = true
			if total <= budget {
				break
			}
		}
		if !dropped {
			break
		}
		// Second pass: drop already-kept middle messages if still over budget
		for i := cfg.PreserveFirst; i < len(messages)-cfg.PreserveLast; i++ {
			if !keep[i] || costs[i] == 0 {
				continue
			}
			total -= costs[i]
			costs[i] = 0
			keep[i] = false
			if total <= budget {
				break
			}
		}
		break
	}

	var result []Message
	for i, m := range messages {
		if keep[i] || costs[i] > 0 {
			result = append(result, m)
		}
	}

	return TrimResult{
		Messages:      result,
		TotalTokens:   total,
		DroppedCount:  original - len(result),
		OriginalCount: original,
	}
}

// EstimateTokens gives a rough token estimate (~4 chars per token for English,
// ~2 chars per token for CJK mixed content).
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	chars := utf8.RuneCountInString(text)
	// Heuristic: CJK-heavy text has ~2 chars/token, Latin ~4 chars/token
	cjkCount := 0
	for _, r := range text {
		if isCJK(r) {
			cjkCount++
		}
	}
	cjkRatio := float64(cjkCount) / float64(chars)
	charsPerToken := 4.0 - 2.0*cjkRatio // ranges from 2 (all CJK) to 4 (all Latin)
	tokens := float64(chars) / charsPerToken
	if tokens < 1 {
		return 1
	}
	return int(tokens) + 4 // +4 for message overhead
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7AF)
}

// PruneToolOutput truncates long tool outputs while preserving head and tail.
func PruneToolOutput(text string, maxBytes int) string {
	if len(text) <= maxBytes || maxBytes <= 0 {
		return text
	}
	headSize := maxBytes * 3 / 4
	tailSize := maxBytes / 4
	lines := strings.Split(text, "\n")

	if len(lines) <= 10 {
		// Short text: just truncate
		return text[:headSize] + "\n...[pruned]...\n" + text[len(text)-tailSize:]
	}

	// Line-based pruning
	headLines := 0
	headBytes := 0
	for headLines < len(lines) && headBytes < headSize {
		headBytes += len(lines[headLines]) + 1
		headLines++
	}

	tailLines := 0
	tailBytes := 0
	for tailLines < len(lines) && tailBytes < tailSize {
		tailBytes += len(lines[len(lines)-1-tailLines]) + 1
		tailLines++
	}

	head := strings.Join(lines[:headLines], "\n")
	tail := strings.Join(lines[len(lines)-tailLines:], "\n")
	dropped := len(lines) - headLines - tailLines

	return head + "\n\n...[" + strings.Repeat(".", 3) + " pruned " +
		string(rune('0'+dropped/100%10)) + string(rune('0'+dropped/10%10)) + string(rune('0'+dropped%10)) +
		" lines ]...\n\n" + tail
}
