package controlplanepack

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/controlplane/models"
)

func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleModelsGet(w, r)
	case http.MethodPost:
		h.handleModelsPost(w, r)
	case http.MethodDelete:
		h.handleModelsDelete(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleModelsGet(w http.ResponseWriter, _ *http.Request) {
	manager := h.gateway.ModelManager()
	if manager == nil {
		writeJSON(w, map[string]any{"models": []models.Entry{}})
		return
	}
	writeJSON(w, map[string]any{"models": manager.List(h.gateway.ProviderModels())})
}

func (h *Handler) handleModelsPost(w http.ResponseWriter, r *http.Request) {
	manager := h.gateway.ModelManager()
	if manager == nil {
		http.Error(w, `{"error":"models unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	var entry models.Entry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if entry.ID == "" || entry.ModelID == "" {
		http.Error(w, `{"error":"id and model_id required"}`, http.StatusBadRequest)
		return
	}
	manager.Upsert(entry)
	writeJSON(w, entry)
}

func (h *Handler) handleModelsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	manager := h.gateway.ModelManager()
	if manager != nil && manager.DeleteExplicit(id) {
		writeJSON(w, map[string]string{"status": "ok"})
		return
	}
	if h.gateway.DeleteProviderModel(id) {
		writeJSON(w, map[string]string{"status": "ok"})
		return
	}
	if manager != nil {
		manager.Hide(id)
	}
	writeJSON(w, map[string]string{"status": "ok"})
}
