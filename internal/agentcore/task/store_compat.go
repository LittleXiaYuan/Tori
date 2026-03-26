package task

// NewStore creates a file-backed task store rooted at dir.
// This is a convenience alias for NewJSONStore.
func NewStore(dir string) *JSONStore {
	return NewJSONStore(dir)
}
