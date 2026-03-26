package emotion

// Flush returns all history entries and clears the history.
func (h *History) Flush() []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	entries := h.entries
	h.entries = make([]HistoryEntry, 0, 128)
	return entries
}
