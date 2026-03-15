package session

import (
	"fmt"
	"sync"
	"time"
)

// ForkMessage represents a message in a forked conversation.
type ForkMessage struct {
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Fork represents a conversation branch point.
type Fork struct {
	ID        string        `json:"id"`
	ParentID  string        `json:"parent_id,omitempty"` // empty = root
	SessionID string        `json:"session_id"`
	Label     string        `json:"label,omitempty"` // user-friendly label
	Messages  []ForkMessage `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	Children  []string      `json:"children,omitempty"` // child fork IDs
}

// ForkTree manages conversation branching for a session.
type ForkTree struct {
	mu    sync.RWMutex
	forks map[string]*Fork // fork_id -> fork
	roots map[string]string // session_id -> root fork_id
	seq   int
}

// NewForkTree creates a fork tree manager.
func NewForkTree() *ForkTree {
	return &ForkTree{
		forks: make(map[string]*Fork),
		roots: make(map[string]string),
	}
}

// Create starts a new root fork for a session (or retrieves existing root).
func (ft *ForkTree) Create(sessionID string, initialMessages []ForkMessage) *Fork {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if rootID, ok := ft.roots[sessionID]; ok {
		if f, ok := ft.forks[rootID]; ok {
			return f.snapshot()
		}
	}

	ft.seq++
	id := fmt.Sprintf("fork_%s_%d", sessionID, ft.seq)
	f := &Fork{
		ID:        id,
		SessionID: sessionID,
		Label:     "main",
		Messages:  copyMessages(initialMessages),
		CreatedAt: time.Now(),
	}
	ft.forks[id] = f
	ft.roots[sessionID] = id
	return f.snapshot()
}

// Branch creates a new fork from an existing one at a specific message index.
// Messages up to (and including) the index are copied; the branch diverges after that point.
func (ft *ForkTree) Branch(forkID string, atMessageIndex int, label string) (*Fork, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	parent, ok := ft.forks[forkID]
	if !ok {
		return nil, fmt.Errorf("fork %s not found", forkID)
	}

	if atMessageIndex < 0 {
		atMessageIndex = len(parent.Messages) - 1
	}
	if atMessageIndex >= len(parent.Messages) {
		atMessageIndex = len(parent.Messages) - 1
	}

	// Copy messages up to the branch point
	branchMessages := copyMessages(parent.Messages[:atMessageIndex+1])

	ft.seq++
	id := fmt.Sprintf("fork_%s_%d", parent.SessionID, ft.seq)
	if label == "" {
		label = fmt.Sprintf("branch-%d", ft.seq)
	}

	child := &Fork{
		ID:        id,
		ParentID:  forkID,
		SessionID: parent.SessionID,
		Label:     label,
		Messages:  branchMessages,
		CreatedAt: time.Now(),
	}
	ft.forks[id] = child
	parent.Children = append(parent.Children, id)

	return child.snapshot(), nil
}

// Append adds a message to a fork.
func (ft *ForkTree) Append(forkID string, msg ForkMessage) error {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	f, ok := ft.forks[forkID]
	if !ok {
		return fmt.Errorf("fork %s not found", forkID)
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	f.Messages = append(f.Messages, msg)
	return nil
}

// Get returns a fork by ID.
func (ft *ForkTree) Get(forkID string) (*Fork, bool) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	f, ok := ft.forks[forkID]
	if !ok {
		return nil, false
	}
	return f.snapshot(), true
}

// GetRoot returns the root fork for a session.
func (ft *ForkTree) GetRoot(sessionID string) (*Fork, bool) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	rootID, ok := ft.roots[sessionID]
	if !ok {
		return nil, false
	}
	f, ok := ft.forks[rootID]
	if !ok {
		return nil, false
	}
	return f.snapshot(), true
}

// ListBranches returns all forks (branches) for a session.
func (ft *ForkTree) ListBranches(sessionID string) []Fork {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	var result []Fork
	for _, f := range ft.forks {
		if f.SessionID == sessionID {
			result = append(result, *f.snapshot())
		}
	}
	return result
}

// Ancestry returns the chain from a fork back to root.
func (ft *ForkTree) Ancestry(forkID string) []Fork {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	var chain []Fork
	current := forkID
	for current != "" {
		f, ok := ft.forks[current]
		if !ok {
			break
		}
		chain = append([]Fork{*f.snapshot()}, chain...)
		current = f.ParentID
	}
	return chain
}

// Delete removes a fork and all its descendants.
func (ft *ForkTree) Delete(forkID string) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	f, ok := ft.forks[forkID]
	if !ok {
		return false
	}

	// Recursively delete children
	ft.deleteRecursive(forkID)

	// Remove from parent's children list
	if f.ParentID != "" {
		if parent, ok := ft.forks[f.ParentID]; ok {
			for i, cid := range parent.Children {
				if cid == forkID {
					parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
					break
				}
			}
		}
	}

	// Remove from roots if it's a root
	if rootID, ok := ft.roots[f.SessionID]; ok && rootID == forkID {
		delete(ft.roots, f.SessionID)
	}

	return true
}

func (ft *ForkTree) deleteRecursive(forkID string) {
	f, ok := ft.forks[forkID]
	if !ok {
		return
	}
	for _, childID := range f.Children {
		ft.deleteRecursive(childID)
	}
	delete(ft.forks, forkID)
}

func (f *Fork) snapshot() *Fork {
	cp := *f
	cp.Messages = copyMessages(f.Messages)
	cp.Children = make([]string, len(f.Children))
	copy(cp.Children, f.Children)
	return &cp
}

func copyMessages(msgs []ForkMessage) []ForkMessage {
	if msgs == nil {
		return nil
	}
	cp := make([]ForkMessage, len(msgs))
	copy(cp, msgs)
	return cp
}
