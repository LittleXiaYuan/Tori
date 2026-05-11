package context

import (
	"math"
	"sort"
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

	// Entropy-aware trimming: drop lowest-entropy messages first.
	// Shannon entropy H = -Σ P(r)·log₂(P(r)) measures information density.
	// Low-entropy messages ("好的", "收到", "OK") are dropped before
	// high-entropy messages (detailed facts, code, analysis).
	type midMsg struct {
		idx     int
		entropy float64
		cost    int
	}
	var mids []midMsg
	for i := cfg.PreserveFirst; i < len(messages)-cfg.PreserveLast; i++ {
		if i >= 0 && i < len(messages) {
			mids = append(mids, midMsg{
				idx:     i,
				entropy: messageEntropy(messages[i].Content),
				cost:    costs[i],
			})
		}
	}
	sort.Slice(mids, func(i, j int) bool {
		return mids[i].entropy < mids[j].entropy
	})

	for _, m := range mids {
		if total <= budget {
			break
		}
		if costs[m.idx] == 0 {
			continue
		}
		total -= costs[m.idx]
		costs[m.idx] = 0
		keep[m.idx] = false
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

// messageEntropy computes normalized Shannon entropy of a message's content.
// Returns a value in [0, 1] where 0 = completely uniform (low info) and
// 1 = maximum diversity (high info). Used by TrimToFit to prioritize
// dropping low-information messages like "好的", "OK", "收到".
func messageEntropy(content string) float64 {
	if len(content) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	total := 0
	for _, r := range content {
		freq[r]++
		total++
	}
	if total <= 1 {
		return 0
	}
	var h float64
	logTotal := math.Log2(float64(total))
	for _, count := range freq {
		p := float64(count) / float64(total)
		h -= p * math.Log2(p)
	}
	if logTotal == 0 {
		return 0
	}
	return h / logTotal
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
