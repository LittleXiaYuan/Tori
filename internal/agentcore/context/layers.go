package context

import (
	"sort"
	"strings"
)

// ──────────────────────────────────────────────
// Context Layering — Priority-based context injection
//
// Each context source registers as a Layer with a priority and
// estimated token cost. The LayerAssembler builds the dynamic
// context string, dropping low-priority layers when the total
// would exceed the budget.
//
// Layer ordering (highest priority first):
//   P0 system     — base system prompt + persona (in stable prefix, not managed here)
//   P1 task       — task working memory (when in task thread)
//   P2 memory     — 5-layer memory recall
//   P3 retrieval  — knowledge graph + RAG context
//   P4 cognition  — emotion, reverie, reflection strategy, state kernel
//   P5 hints      — skill optimization hints, code context
// ──────────────────────────────────────────────

// LayerPriority defines injection priority (lower = higher priority).
type LayerPriority int

const (
	LayerPriorityTask      LayerPriority = 10 // task working memory
	LayerPriorityMemory    LayerPriority = 20 // memory recall
	LayerPriorityRetrieval LayerPriority = 30 // knowledge graph / RAG
	LayerPriorityCognition LayerPriority = 40 // emotion, reverie, state, strategy
	LayerPriorityHints     LayerPriority = 50 // optimization hints, code context
)

// Layer is a single context injection source.
type Layer struct {
	Name     string        // human-readable name (e.g. "memory", "task", "emotion")
	Priority LayerPriority // lower = higher priority
	Content  string        // rendered content
	Tokens   int           // estimated token count (0 = auto-estimate)
}

// LayerAssembler collects context layers and assembles them within a budget.
type LayerAssembler struct {
	maxTokens    int // 0 = unlimited
	maxPerLayer  int // 0 = no per-layer limit; max tokens any single layer may consume
	dedup        bool // if true, detect and remove duplicate sentences across layers
}

// NewLayerAssembler creates an assembler with optional max token budget for dynamic context.
func NewLayerAssembler(maxDynTokens int) *LayerAssembler {
	return &LayerAssembler{maxTokens: maxDynTokens}
}

// WithPerLayerLimit sets a maximum token budget per individual layer.
// Layers exceeding this are truncated (not dropped), preserving partial context.
func (la *LayerAssembler) WithPerLayerLimit(maxPerLayer int) *LayerAssembler {
	la.maxPerLayer = maxPerLayer
	return la
}

// WithDedup enables cross-layer sentence deduplication.
func (la *LayerAssembler) WithDedup() *LayerAssembler {
	la.dedup = true
	return la
}

// Assemble takes a set of layers, sorts by priority, and returns the assembled
// context string. Low-priority layers are dropped if the token budget is exceeded.
// Returns the assembled string and the list of included layer names.
func (la *LayerAssembler) Assemble(layers []Layer) (string, []string) {
	// Filter out empty layers
	var active []Layer
	for _, l := range layers {
		if l.Content == "" {
			continue
		}
		if l.Tokens <= 0 {
			l.Tokens = estimateTokens(l.Content)
		}
		// Per-layer truncation: truncate oversized layers instead of dropping them
		if la.maxPerLayer > 0 && l.Tokens > la.maxPerLayer {
			l.Content = truncateToTokens(l.Content, la.maxPerLayer)
			l.Tokens = la.maxPerLayer
		}
		active = append(active, l)
	}

	// Sort by priority (lowest number = highest priority)
	sort.Slice(active, func(i, j int) bool {
		return active[i].Priority < active[j].Priority
	})

	// Cross-layer dedup: track seen sentences, remove duplicates from lower-priority layers
	var seen map[string]bool
	if la.dedup {
		seen = make(map[string]bool)
	}

	// Accumulate within budget
	var parts []string
	var included []string
	totalTokens := 0
	for _, l := range active {
		content := l.Content
		tokens := l.Tokens

		if la.dedup && seen != nil {
			content, tokens = dedupContent(content, seen)
			if content == "" {
				continue
			}
		}

		if la.maxTokens > 0 && totalTokens+tokens > la.maxTokens {
			// Try to fit a partial layer instead of dropping entirely
			remaining := la.maxTokens - totalTokens
			if remaining > 50 {
				content = truncateToTokens(content, remaining)
				tokens = remaining
			} else {
				continue
			}
		}
		parts = append(parts, content)
		included = append(included, l.Name)
		totalTokens += tokens
	}

	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, "\n\n"), included
}

// truncateToTokens truncates content to approximately maxTokens.
func truncateToTokens(content string, maxTokens int) string {
	runes := []rune(content)
	maxChars := maxTokens * 35 / 10 // inverse of estimate: ~3.5 chars per token
	if len(runes) <= maxChars {
		return content
	}
	return string(runes[:maxChars]) + "\n...[截断]"
}

// dedupContent removes sentences that have already been seen in higher-priority layers.
// Returns the deduplicated content and its new token estimate.
func dedupContent(content string, seen map[string]bool) (string, int) {
	lines := strings.Split(content, "\n")
	var kept []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "[") {
			kept = append(kept, line)
			continue
		}
		if len(trimmed) < 10 {
			kept = append(kept, line)
			continue
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		kept = append(kept, line)
	}
	result := strings.Join(kept, "\n")
	return strings.TrimSpace(result), estimateTokens(result)
}

// estimateTokens estimates token count (~3.5 chars per token for mixed CJK/EN).
func estimateTokens(s string) int {
	r := []rune(s)
	return len(r)*10/35 + 1 // ≈ 3.5 chars per token, rounded up
}
