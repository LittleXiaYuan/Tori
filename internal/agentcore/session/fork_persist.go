package session

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
)

// forkSnapshot is the serializable state of a ForkTree.
type forkSnapshot struct {
	Forks map[string]*Fork  `json:"forks"`
	Roots map[string]string `json:"roots"`
	Seq   int               `json:"seq"`
}

// ForkPersister saves/loads ForkTree state to a JSON file.
type ForkPersister struct {
	mu   sync.Mutex
	path string
}

// NewForkPersister creates a persister for the given file path.
func NewForkPersister(path string) *ForkPersister {
	return &ForkPersister{path: path}
}

// Save writes the fork tree state to disk.
func (p *ForkPersister) Save(ft *ForkTree) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ft.mu.RLock()
	snap := forkSnapshot{
		Forks: ft.forks,
		Roots: ft.roots,
		Seq:   ft.seq,
	}
	ft.mu.RUnlock()

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.path, data, 0644)
}

// Load restores a fork tree from disk.
func (p *ForkPersister) Load(ft *ForkTree) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no saved state
		}
		return err
	}

	var snap forkSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	ft.mu.Lock()
	defer ft.mu.Unlock()
	if snap.Forks != nil {
		ft.forks = snap.Forks
	}
	if snap.Roots != nil {
		ft.roots = snap.Roots
	}
	ft.seq = snap.Seq

	slog.Info("fork tree loaded", "forks", len(ft.forks), "roots", len(ft.roots))
	return nil
}
