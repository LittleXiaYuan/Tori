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
	maxTokens int // 0 = unlimited
}

// NewLayerAssembler creates an assembler with optional max token budget for dynamic context.
func NewLayerAssembler(maxDynTokens int) *LayerAssembler {
	return &LayerAssembler{maxTokens: maxDynTokens}
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
		active = append(active, l)
	}

	// Sort by priority (lowest number = highest priority)
	sort.Slice(active, func(i, j int) bool {
		return active[i].Priority < active[j].Priority
	})

	// Accumulate within budget
	var parts []string
	var included []string
	totalTokens := 0
	for _, l := range active {
		if la.maxTokens > 0 && totalTokens+l.Tokens > la.maxTokens {
			continue // skip this layer — over budget
		}
		parts = append(parts, l.Content)
		included = append(included, l.Name)
		totalTokens += l.Tokens
	}

	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, "\n\n"), included
}

// estimateTokens estimates token count (~3.5 chars per token for mixed CJK/EN).
func estimateTokens(s string) int {
	r := []rune(s)
	return len(r)*10/35 + 1 // ≈ 3.5 chars per token, rounded up
}
