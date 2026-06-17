package models

import (
	"context"
	"log/slog"
	"sync"
)

type Entry struct {
	ID                string `json:"id"`
	ModelID           string `json:"model_id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	ClientType        string `json:"client_type"`
	BaseURL           string `json:"base_url,omitempty"`
	SupportsReasoning bool   `json:"supports_reasoning"`
	Dimensions        int    `json:"dimensions,omitempty"`
}

type KVStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

type ProviderModel struct {
	ID      string
	Model   string
	Type    string
	BaseURL string
}

type Manager struct {
	mu     sync.RWMutex
	models map[string]Entry
	hidden map[string]bool // provider-derived model IDs the user explicitly deleted
	kvs    KVStore
}

func NewManager() *Manager {
	return &Manager{
		models: make(map[string]Entry),
		hidden: make(map[string]bool),
	}
}

func (m *Manager) SetKVStore(kvs KVStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kvs = kvs

	var entries []Entry
	if found, err := kvs.Get(context.Background(), "models", &entries); err == nil && found {
		for _, entry := range entries {
			m.models[entry.ID] = entry
		}
		slog.Info("models: loaded from KV", "count", len(entries))
	}

	var hidden []string
	if found, err := kvs.Get(context.Background(), "models_hidden", &hidden); err == nil && found {
		for _, id := range hidden {
			m.hidden[id] = true
		}
		if len(hidden) > 0 {
			slog.Info("models: loaded hidden list from KV", "count", len(hidden))
		}
	}
}

func (m *Manager) List(providers []ProviderModel) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]Entry, 0, len(m.models)+len(providers))
	existing := make(map[string]bool, len(m.models))
	for _, entry := range m.models {
		entries = append(entries, entry)
		existing[entry.ModelID] = true
	}

	for _, provider := range providers {
		syntheticID := provider.ID + "-" + provider.Model
		if provider.Model != "" && !existing[provider.Model] && !m.hidden[syntheticID] {
			entries = append(entries, Entry{
				ID:         syntheticID,
				ModelID:    provider.Model,
				Name:       provider.Model,
				Type:       provider.Type,
				ClientType: provider.Type,
				BaseURL:    provider.BaseURL,
			})
		}
	}
	return entries
}

func (m *Manager) Upsert(entry Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.models[entry.ID] = entry
	delete(m.hidden, entry.ID)
	m.persistModelsLocked()
	m.persistHiddenLocked()
}

func (m *Manager) DeleteExplicit(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.models[id]; !ok {
		return false
	}
	delete(m.models, id)
	m.persistModelsLocked()
	return true
}

func (m *Manager) Hide(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hidden[id] = true
	m.persistHiddenLocked()
}

func (m *Manager) persistModelsLocked() {
	if m.kvs == nil {
		return
	}
	entries := make([]Entry, 0, len(m.models))
	for _, entry := range m.models {
		entries = append(entries, entry)
	}
	if err := m.kvs.Put(context.Background(), "models", entries); err != nil {
		slog.Error("models: persist models failed", "err", err)
	}
}

func (m *Manager) persistHiddenLocked() {
	if m.kvs == nil {
		return
	}
	ids := make([]string, 0, len(m.hidden))
	for id := range m.hidden {
		ids = append(ids, id)
	}
	if err := m.kvs.Put(context.Background(), "models_hidden", ids); err != nil {
		slog.Error("models: persist hidden failed", "err", err)
	}
}
