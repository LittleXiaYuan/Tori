package emotion

import (
	"sync"
	"time"
)

// HistoryEntry records a single emotion detection event.
type HistoryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	SessionID  string    `json:"session_id"`
	Emotion    Emotion   `json:"emotion"`
	Confidence float64   `json:"confidence"`
	Source     string    `json:"source"` // "text", "audio", etc.
}

// History stores emotion detection events in memory with a configurable cap.
type History struct {
	mu      sync.RWMutex
	entries []HistoryEntry
	maxSize int
}

// NewHistory creates a new history store with the given max capacity.
func NewHistory(maxSize int) *History {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &History{entries: make([]HistoryEntry, 0, 128), maxSize: maxSize}
}

// Record adds an emotion event to the history.
func (h *History) Record(sessionID string, e Emotion, confidence float64, source string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, HistoryEntry{
		Timestamp:  time.Now(),
		SessionID:  sessionID,
		Emotion:    e,
		Confidence: confidence,
		Source:     source,
	})
	if len(h.entries) > h.maxSize {
		h.entries = h.entries[len(h.entries)-h.maxSize:]
	}
}

// Query returns history entries matching the given filters.
// Pass empty sessionID to match all sessions. Zero times mean no bound.
func (h *History) Query(sessionID string, from, to time.Time, limit int) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []HistoryEntry
	for i := len(h.entries) - 1; i >= 0; i-- {
		e := h.entries[i]
		if sessionID != "" && e.SessionID != sessionID {
			continue
		}
		if !from.IsZero() && e.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && e.Timestamp.After(to) {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	// Reverse to chronological order.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Summary returns emotion frequency counts for the given entries.
func Summary(entries []HistoryEntry) map[Emotion]int {
	counts := make(map[Emotion]int)
	for _, e := range entries {
		counts[e.Emotion]++
	}
	return counts
}
