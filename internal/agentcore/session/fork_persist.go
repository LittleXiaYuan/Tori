package session

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"
)

// kvStore abstracts Ledger KV to avoid import cycles with internal/ledger.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// forkSnapshot is the serializable state of a ForkTree.
type forkSnapshot struct {
	Forks map[string]*Fork  `json:"forks"`
	Roots map[string]string `json:"roots"`
	Seq   int               `json:"seq"`
}

// ForkPersister saves/loads ForkTree state to JSON file or Ledger KV.
type ForkPersister struct {
	mu   sync.Mutex
	path string
	kvs  kvStore
}

// NewForkPersister creates a persister for the given file path.
func NewForkPersister(path string) *ForkPersister {
	return &ForkPersister{path: path}
}

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
func (p *ForkPersister) SetKVStore(kvs kvStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.kvs = kvs
}

// Save writes the fork tree state to Ledger KV (preferred) or disk (fallback).
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

	if p.kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.kvs.Put(ctx, "forks", snap); err != nil {
			slog.Error("fork persister: KV save failed, falling back to file", "err", err)
		} else {
			return nil
		}
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.path, data, 0644)
}

// Load restores a fork tree from Ledger KV (preferred) or disk (fallback).
func (p *ForkPersister) Load(ft *ForkTree) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.kvs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var snap forkSnapshot
		found, err := p.kvs.Get(ctx, "forks", &snap)
		if err != nil {
			slog.Warn("fork persister: KV load failed, falling back to file", "err", err)
		} else if found {
			ft.mu.Lock()
			defer ft.mu.Unlock()
			if snap.Forks != nil {
				ft.forks = snap.Forks
			}
			if snap.Roots != nil {
				ft.roots = snap.Roots
			}
			ft.seq = snap.Seq
			slog.Info("fork tree loaded from KV", "forks", len(ft.forks), "roots", len(ft.roots))
			return nil
		}
	}

	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
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
