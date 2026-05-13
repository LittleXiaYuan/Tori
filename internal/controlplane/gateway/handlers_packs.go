package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"yunque-agent/pkg/packruntime"
)

type packActionRequest struct {
	ID string `json:"id"`
}

type packInstallRequest struct {
	ManifestPath string `json:"manifest_path"`
	ManifestURL  string `json:"manifest_url"`
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
	g.registerBackupPackRoutes()
}

func (g *Gateway) registerBackupPackRoutes() {
	g.mux.HandleFunc("/v1/backup/export", g.requireAuth(g.requirePackRoute("yunque.pack.backup", "/v1/backup/export", g.handleBackupExport)))
	g.mux.HandleFunc("/v1/backup/import", g.requireAuth(g.requirePackRoute("yunque.pack.backup", "/v1/backup/import", g.handleBackupImport)))
	g.mux.HandleFunc("/v1/backup/info", g.requireAuth(g.requirePackRoute("yunque.pack.backup", "/v1/backup/info", g.handleBackupInfo)))
}

func (g *Gateway) requirePackRoute(packID string, route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.packRouteEnabled(packID, route) {
			writeJSONStatus(w, http.StatusNotFound, map[string]any{
				"error":   "pack route is not enabled",
				"pack_id": packID,
				"route":   route,
			})
			return
		}
		next(w, r)
	}
}

func (g *Gateway) packRouteEnabled(packID string, route string) bool {
	if g.packRegistry == nil {
		return false
	}
	pack, ok := g.packRegistry.Get(packID)
	if !ok || pack.Status != packruntime.PackStatusEnabled {
		return false
	}
	route = strings.TrimSpace(route)
	return route != "" && slices.Contains(pack.Manifest.Backend.Routes, route)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.ManifestPath == "" && req.ManifestURL == "") {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "manifest_path or manifest_url is required"})
		return
	}
	manifest, source, err := loadPackInstallManifest(r, req)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	pack, err := g.packRegistry.Install(manifest, source)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"pack": pack, "status": pack.Status})
}

func loadPackInstallManifest(r *http.Request, req packInstallRequest) (packruntime.Manifest, string, error) {
	source := req.Source
	if req.ManifestURL != "" {
		manifest, err := fetchPackManifest(r, req.ManifestURL)
		if err != nil {
			return packruntime.Manifest{}, "", err
		}
		if source == "" {
			source = req.ManifestURL
		}
		return manifest, source, nil
	}
	manifest, err := packruntime.LoadManifest(req.ManifestPath)
	if err != nil {
		return packruntime.Manifest{}, "", err
	}
	if source == "" {
		source = filepath.Dir(req.ManifestPath)
	}
	return manifest, source, nil
}

func fetchPackManifest(r *http.Request, manifestURL string) (packruntime.Manifest, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, manifestURL, nil)
	if err != nil {
		return packruntime.Manifest{}, fmt.Errorf("create pack manifest request: %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return packruntime.Manifest{}, fmt.Errorf("download pack manifest: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return packruntime.Manifest{}, fmt.Errorf("download pack manifest: http %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return packruntime.Manifest{}, fmt.Errorf("read downloaded pack manifest: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return packruntime.Manifest{}, fmt.Errorf("downloaded pack manifest is empty")
	}
	var manifest packruntime.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return packruntime.Manifest{}, fmt.Errorf("parse downloaded pack manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return packruntime.Manifest{}, err
	}
	return manifest, nil
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
