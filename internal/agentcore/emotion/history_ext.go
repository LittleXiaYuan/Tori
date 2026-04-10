package emotion

// Flush persists entries to KV (if available) and clears in-memory history.
// Returns the flushed entries for callers that need them.
func (h *History) Flush() []HistoryEntry {
	h.FlushToKV()

	h.mu.Lock()
	defer h.mu.Unlock()
	entries := h.entries
	h.entries = make([]HistoryEntry, 0, 128)
	h.dirty = 0
	return entries
}
