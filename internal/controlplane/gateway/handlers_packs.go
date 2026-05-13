package gateway

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"yunque-agent/pkg/packruntime"
)

type packActionRequest struct {
	ID string `json:"id"`
}

type packInstallRequest struct {
	ManifestPath string `json:"manifest_path"`
	Source       string `json:"source"`
}

func (g *Gateway) registerPackRoutes() {
	g.mux.HandleFunc("/v1/packs", g.requireAuth(g.handlePacksList))
	g.mux.HandleFunc("/v1/packs/installed", g.requireAuth(g.handlePacksList))
	g.mux.HandleFunc("/v1/packs/enabled", g.requireAuth(g.handlePacksEnabled))
	g.mux.HandleFunc("/v1/packs/install", g.requireAuth(g.handlePackInstall))
	g.mux.HandleFunc("/v1/packs/enable", g.requireAuth(g.handlePackEnable))
	g.mux.HandleFunc("/v1/packs/disable", g.requireAuth(g.handlePackDisable))
	g.mux.HandleFunc("/v1/packs/rollback", g.requireAuth(g.handlePackRollback))
}

func (g *Gateway) handlePacksList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	registry := g.packRegistry
	if registry == nil {
		writeJSON(w, map[string]any{"packs": []packruntime.InstalledPack{}, "enabled": []packruntime.InstalledPack{}, "count": 0})
		return
	}
	packs := registry.List()
	writeJSON(w, map[string]any{"packs": packs, "enabled": registry.Enabled(), "count": len(packs)})
}

func (g *Gateway) handlePacksEnabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	registry := g.packRegistry
	if registry == nil {
		writeJSON(w, map[string]any{"packs": []packruntime.InstalledPack{}, "count": 0})
		return
	}
	packs := registry.Enabled()
	writeJSON(w, map[string]any{"packs": packs, "count": len(packs)})
}

func (g *Gateway) handlePackInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if g.packRegistry == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "pack registry not configured"})
		return
	}
	var req packInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ManifestPath == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "manifest_path is required"})
		return
	}
	manifest, err := packruntime.LoadManifest(req.ManifestPath)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := req.Source
	if source == "" {
		source = filepath.Dir(req.ManifestPath)
	}
	pack, err := g.packRegistry.Install(manifest, source)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"pack": pack, "status": pack.Status})
}

func (g *Gateway) handlePackEnable(w http.ResponseWriter, r *http.Request) {
	g.handlePackMutation(w, r, func(registry *packruntime.Registry, id string) (packruntime.InstalledPack, error) {
		return registry.Enable(id)
	})
}

func (g *Gateway) handlePackDisable(w http.ResponseWriter, r *http.Request) {
	g.handlePackMutation(w, r, func(registry *packruntime.Registry, id string) (packruntime.InstalledPack, error) {
		return registry.Disable(id)
	})
}

func (g *Gateway) handlePackRollback(w http.ResponseWriter, r *http.Request) {
	g.handlePackMutation(w, r, func(registry *packruntime.Registry, id string) (packruntime.InstalledPack, error) {
		return registry.Rollback(id)
	})
}

func (g *Gateway) handlePackMutation(w http.ResponseWriter, r *http.Request, mutate func(*packruntime.Registry, string) (packruntime.InstalledPack, error)) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if g.packRegistry == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "pack registry not configured"})
		return
	}
	var req packActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	pack, err := mutate(g.packRegistry, req.ID)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"pack": pack, "status": pack.Status})
}
