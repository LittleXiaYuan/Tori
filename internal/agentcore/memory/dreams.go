package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// DreamEntry represents a cross-conversation pattern or insight.
type DreamEntry struct {
	ID        string   `json:"id"`
	Pattern   string   `json:"pattern"`
	Insight   string   `json:"insight"`
	Frequency int      `json:"frequency"`
	Topics    []string `json:"topics,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// DreamResult is the output of a dreaming consolidation cycle.
type DreamResult struct {
	Entries     []DreamEntry `json:"entries"`
	NewPatterns int          `json:"new_patterns"`
	Duration    string       `json:"duration"`
}

// DreamConsolidator performs cross-conversation pattern recognition.
type DreamConsolidator struct {
	chat func(ctx context.Context, prompt string) (string, error)
}

func NewDreamConsolidator(chatFn func(ctx context.Context, prompt string) (string, error)) *DreamConsolidator {
	return &DreamConsolidator{chat: chatFn}
}

// Consolidate analyzes recent conversation summaries and extracts patterns.
func (dc *DreamConsolidator) Consolidate(ctx context.Context, conversationSummaries []string, existingDreams []DreamEntry) (*DreamResult, error) {
	start := time.Now()

	if len(conversationSummaries) < 2 {
		return &DreamResult{Entries: existingDreams}, nil
	}

	combined := strings.Join(conversationSummaries, "\n---\n")
	if len(combined) > 16000 {
		combined = combined[:16000] + "\n...(truncated)"
	}

	existingJSON, _ := json.Marshal(existingDreams)
	existingStr := ""
	if len(existingDreams) > 0 {
		existingStr = fmt.Sprintf("\n\nPreviously identified patterns:\n%s", string(existingJSON))
	}

	prompt := fmt.Sprintf(`You are a cognitive pattern analyzer. Analyze these conversation summaries and identify recurring patterns, trends, and insights across conversations.

For each pattern found, create a JSON object:
- "id": short unique identifier (lowercase, underscore-separated)
- "pattern": concise description of the recurring pattern
- "insight": actionable insight derived from the pattern
- "frequency": estimated occurrence count across conversations
- "topics": related topic keywords

If existing patterns are provided, update their frequency and insights rather than duplicating.
Return a JSON array. Focus on genuinely useful patterns, not trivial observations. 3-8 entries max.
Write in the same language as the conversation content.

Conversation summaries:
%s%s`, combined, existingStr)

	reply, err := dc.chat(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("dream consolidation: %w", err)
	}

	reply = strings.TrimSpace(reply)
	if idx := strings.Index(reply, "["); idx >= 0 {
		reply = reply[idx:]
	}
	if idx := strings.LastIndex(reply, "]"); idx >= 0 {
		reply = reply[:idx+1]
	}

	var entries []DreamEntry
	if err := json.Unmarshal([]byte(reply), &entries); err != nil {
		slog.Warn("dreams: parse failed, keeping existing", "err", err)
		return &DreamResult{Entries: existingDreams}, nil
	}

	now := time.Now().Format(time.RFC3339)
	for i := range entries {
		if entries[i].CreatedAt == "" {
			entries[i].CreatedAt = now
		}
		entries[i].UpdatedAt = now
	}

	newCount := 0
	existingIDs := make(map[string]bool)
	for _, e := range existingDreams {
		existingIDs[e.ID] = true
	}
	for _, e := range entries {
		if !existingIDs[e.ID] {
			newCount++
		}
	}

	result := &DreamResult{
		Entries:     entries,
		NewPatterns: newCount,
		Duration:    time.Since(start).String(),
	}
	slog.Info("dreams: consolidation complete", "patterns", len(entries), "new", newCount)
	return result, nil
}
