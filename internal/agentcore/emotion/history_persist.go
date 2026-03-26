package emotion

// SetPersistFile sets the file path for periodic persistence of emotion history.
func (h *History) SetPersistFile(path string) error {
	// Persistence is handled by the caller / lifecycle manager.
	// This is a no-op placeholder for forward compatibility.
	return nil
}
