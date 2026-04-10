package gateway

import (
	"encoding/json"
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

var (
	modelStore   = make(map[string]modelEntry)
	modelStoreMu sync.RWMutex
)

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
	modelStoreMu.RLock()
	defer modelStoreMu.RUnlock()

	models := make([]modelEntry, 0, len(modelStore))
	for _, m := range modelStore {
		models = append(models, m)
	}

	if g.providerReg != nil {
		existing := make(map[string]bool)
		for _, m := range models {
			existing[m.ModelID] = true
		}
		for _, p := range g.providerReg.List() {
			if p.Model != "" && !existing[p.Model] {
				models = append(models, modelEntry{
					ID:         p.ID + "-" + p.Model,
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

	modelStoreMu.Lock()
	modelStore[m.ID] = m
	modelStoreMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

func (g *Gateway) handleModelsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}

	modelStoreMu.Lock()
	_, inStore := modelStore[id]
	if inStore {
		delete(modelStore, id)
	}
	modelStoreMu.Unlock()

	if !inStore && g.providerReg != nil {
		for _, p := range g.providerReg.List() {
			syntheticID := p.ID + "-" + p.Model
			if syntheticID == id || p.ID == id {
				_ = g.providerReg.Delete(p.ID)
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
