package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
)

type modelEntry struct {
	ID                string `json:"id"`
	ModelID           string `json:"model_id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	ClientType        string `json:"client_type"`
	BaseURL           string `json:"base_url,omitempty"`
	SupportsReasoning bool   `json:"supports_reasoning"`
	Dimensions        int    `json:"dimensions,omitempty"`
}

type modelKVStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

type modelManager struct {
	mu      sync.RWMutex
	models  map[string]modelEntry
	hidden  map[string]bool // provider-derived model IDs the user explicitly deleted
	kvs     modelKVStore
}

func newModelManager() *modelManager {
	return &modelManager{
		models: make(map[string]modelEntry),
		hidden: make(map[string]bool),
	}
}

func (mm *modelManager) SetKVStore(kvs modelKVStore) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.kvs = kvs
	var models []modelEntry
	if found, err := kvs.Get(context.Background(), "models", &models); err == nil && found {
		for _, m := range models {
			mm.models[m.ID] = m
		}
		slog.Info("models: loaded from KV", "count", len(models))
	}
	var hidden []string
	if found, err := kvs.Get(context.Background(), "models_hidden", &hidden); err == nil && found {
		for _, id := range hidden {
			mm.hidden[id] = true
		}
		if len(hidden) > 0 {
			slog.Info("models: loaded hidden list from KV", "count", len(hidden))
		}
	}
}

func (mm *modelManager) persistModels() {
	if mm.kvs == nil {
		return
	}
	models := make([]modelEntry, 0, len(mm.models))
	for _, m := range mm.models {
		models = append(models, m)
	}
	if err := mm.kvs.Put(context.Background(), "models", models); err != nil {
		slog.Error("models: persist models failed", "err", err)
	}
}

func (mm *modelManager) persistHidden() {
	if mm.kvs == nil {
		return
	}
	ids := make([]string, 0, len(mm.hidden))
	for id := range mm.hidden {
		ids = append(ids, id)
	}
	if err := mm.kvs.Put(context.Background(), "models_hidden", ids); err != nil {
		slog.Error("models: persist hidden failed", "err", err)
	}
}

func (g *Gateway) handleModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.handleModelsGet(w, r)
	case http.MethodPost:
		g.handleModelsPost(w, r)
	case http.MethodDelete:
		g.handleModelsDelete(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (g *Gateway) handleModelsGet(w http.ResponseWriter, _ *http.Request) {
	mm := g.modelMgr
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	models := make([]modelEntry, 0, len(mm.models))
	for _, m := range mm.models {
		models = append(models, m)
	}

	if g.providerReg != nil {
		existing := make(map[string]bool)
		for _, m := range models {
			existing[m.ModelID] = true
		}
		for _, p := range g.providerReg.List() {
			syntheticID := p.ID + "-" + p.Model
			if p.Model != "" && !existing[p.Model] && !mm.hidden[syntheticID] {
				models = append(models, modelEntry{
					ID:         syntheticID,
					ModelID:    p.Model,
					Name:       p.Model,
					Type:       string(p.Type),
					ClientType: string(p.Type),
					BaseURL:    p.BaseURL,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"models": models})
}

func (g *Gateway) handleModelsPost(w http.ResponseWriter, r *http.Request) {
	var m modelEntry
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if m.ID == "" || m.ModelID == "" {
		http.Error(w, `{"error":"id and model_id required"}`, http.StatusBadRequest)
		return
	}

	mm := g.modelMgr
	mm.mu.Lock()
	mm.models[m.ID] = m
	delete(mm.hidden, m.ID)
	mm.persistModels()
	mm.persistHidden()
	mm.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

func (g *Gateway) handleModelsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	mm := g.modelMgr
	mm.mu.Lock()
	_, inStore := mm.models[id]
	if inStore {
		delete(mm.models, id)
		mm.persistModels()
	}
	mm.mu.Unlock()

	if !inStore && g.providerReg != nil {
		deleted := false
		for _, p := range g.providerReg.List() {
			syntheticID := p.ID + "-" + p.Model
			if syntheticID == id || p.ID == id {
				_ = g.providerReg.Delete(p.ID)
				deleted = true
				break
			}
		}
		if !deleted {
			mm.mu.Lock()
			mm.hidden[id] = true
			mm.persistHidden()
			mm.mu.Unlock()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
