package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/orchestrator"
)

func (g *Gateway) registerProjectRoutes() {
	if g.projectStore == nil {
		g.projectStore = orchestrator.NewProjectStore("data/projects")
	}
	g.mux.HandleFunc("/v1/projects", g.requireAuth(g.handleProjects))
	g.mux.HandleFunc("/v1/projects/detail", g.requireAuth(g.handleProjectDetail))
	g.mux.HandleFunc("/v1/projects/remove", g.requireAuth(g.handleProjectRemove))
}

func (g *Gateway) handleProjects(w http.ResponseWriter, r *http.Request) {
	if g.projectStore == nil {
		writeJSONStatus(w, 503, map[string]string{"error": "project store not available"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		list := g.projectStore.List()
		writeJSON(w, map[string]any{"projects": list})

	case http.MethodPost:
		var req orchestrator.CreateProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONStatus(w, 400, map[string]string{"error": err.Error()})
			return
		}
		p, err := g.projectStore.Create(req)
		if err != nil {
			writeJSONStatus(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSONStatus(w, 201, p)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (g *Gateway) handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	if g.projectStore == nil {
		writeJSONStatus(w, 503, map[string]string{"error": "project store not available"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSONStatus(w, 400, map[string]string{"error": "id required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		p, ok := g.projectStore.Get(id)
		if !ok {
			writeJSONStatus(w, 404, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, p)

	case http.MethodPut:
		p, ok := g.projectStore.Get(id)
		if !ok {
			writeJSONStatus(w, 404, map[string]string{"error": "not found"})
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
			writeJSONStatus(w, 400, map[string]string{"error": err.Error()})
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
		if err := g.projectStore.Update(p); err != nil {
			writeJSONStatus(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, p)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (g *Gateway) handleProjectRemove(w http.ResponseWriter, r *http.Request) {
	if g.projectStore == nil {
		writeJSONStatus(w, 503, map[string]string{"error": "project store not available"})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSONStatus(w, 400, map[string]string{"error": "id required"})
		return
	}
	if !g.projectStore.Delete(body.ID) {
		writeJSONStatus(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}
