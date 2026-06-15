package workpack

// projects.go holds the de-shelled project handlers (/v1/projects*) that moved
// out of the gateway into this pack. They reach the project store through the
// narrow WorkGateway.ProjectStore() accessor.

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/orchestrator"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) handleProjects(w http.ResponseWriter, r *http.Request) {
	store := h.gateway.ProjectStore()
	if store == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "project store not available"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"projects": store.List()})

	case http.MethodPost:
		var req orchestrator.CreateProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		p, err := store.Create(req)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSONStatus(w, http.StatusCreated, p)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	store := h.gateway.ProjectStore()
	if store == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "project store not available"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, ok := store.Get(id)
		if !ok {
			writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, p)

	case http.MethodPut:
		p, ok := store.Get(id)
		if !ok {
			writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var upd struct {
			Name        *string           `json:"name"`
			RepoPath    *string           `json:"repo_path"`
			RepoURL     *string           `json:"repo_url"`
			Description *string           `json:"description"`
			DefaultCaps []string          `json:"default_caps"`
			Meta        map[string]string `json:"meta"`
		}
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if upd.Name != nil {
			p.Name = *upd.Name
		}
		if upd.RepoPath != nil {
			p.RepoPath = *upd.RepoPath
		}
		if upd.RepoURL != nil {
			p.RepoURL = *upd.RepoURL
		}
		if upd.Description != nil {
			p.Description = *upd.Description
		}
		if upd.DefaultCaps != nil {
			p.DefaultCaps = upd.DefaultCaps
		}
		if upd.Meta != nil {
			p.Meta = upd.Meta
		}
		if err := store.Update(p); err != nil {
			writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, p)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleProjectRemove(w http.ResponseWriter, r *http.Request) {
	store := h.gateway.ProjectStore()
	if store == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "project store not available"})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	if !store.Delete(body.ID) {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}
