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
	"time"

	backuppack "yunque-agent/internal/packs/backup"
	"yunque-agent/pkg/packruntime"
)

type packActionRequest struct {
	ID string `json:"id"`
}

type packInstallRequest struct {
	ManifestPath string `json:"manifest_path"`
	ManifestURL  string `json:"manifest_url"`
	Source       string `json:"source"`
	Download     bool   `json:"download"`
}

func (g *Gateway) registerPackRoutes() {
	g.mux.HandleFunc("/v1/packs", g.requireAuth(g.handlePacksList))
	g.mux.HandleFunc("/v1/packs/installed", g.requireAuth(g.handlePacksList))
	g.mux.HandleFunc("/v1/packs/enabled", g.requireAuth(g.handlePacksEnabled))
	g.mux.HandleFunc("/v1/packs/backend-modules", g.requireAuth(g.handlePackBackendModules))
	g.mux.HandleFunc("/v1/packs/backend-route-audit", g.requireAuth(g.handlePackBackendRouteAudit))
	g.mux.HandleFunc("/v1/packs/install", g.requireAuth(g.handlePackInstall))
	g.mux.HandleFunc("/v1/packs/enable", g.requireAuth(g.handlePackEnable))
	g.mux.HandleFunc("/v1/packs/disable", g.requireAuth(g.handlePackDisable))
	g.mux.HandleFunc("/v1/packs/rollback", g.requireAuth(g.handlePackRollback))
	g.mux.HandleFunc("/v1/packs/prune", g.requireAuth(g.handlePackPrune))
	g.registerBuiltinBackendPacks()
}

func (g *Gateway) registerBuiltinBackendPacks() {
	if len(g.backendPacks) == 0 {
		g.RegisterBackendPack(backuppack.DefaultHandler())
		return
	}
	for _, module := range g.backendPacks {
		g.registerBackendPack(module)
	}
}

func (g *Gateway) registerBackendPack(module packruntime.BackendModule) {
	if module == nil {
		return
	}
	packID := module.PackID()
	g.routesMu.Lock()
	defer g.routesMu.Unlock()
	if g.backendPackRoutes == nil {
		g.backendPackRoutes = make(map[string]string)
	}
	if g.backendPackRouteInfos == nil {
		g.backendPackRouteInfos = make(map[string]packruntime.BackendRouteInfo)
	}
	for _, route := range module.Routes() {
		route := route
		route.Path = strings.TrimSpace(route.Path)
		methods := normalizeBackendRouteMethods(route)
		if route.Path == "" || route.Handler == nil {
			continue
		}
		if len(methods) == 0 {
			panic(fmt.Sprintf("backend pack route %s from %s must declare an HTTP method", route.Path, packID))
		}
		if owner, ok := g.backendPackRoutes[route.Path]; ok {
			if owner == packID {
				continue
			}
			panic(fmt.Sprintf("backend pack route conflict: %s already registered by %s, cannot register %s", route.Path, owner, packID))
		}
		g.backendPackRoutes[route.Path] = packID
		g.backendPackRouteInfos[route.Path] = packruntime.BackendRouteInfo{Method: methods[0], Methods: methods, Path: route.Path, Auth: string(route.Auth)}
		authed := g.backendPackAuth(route.Auth, g.requirePackRoute(packID, methods, route.Path, func(w http.ResponseWriter, r *http.Request) {
			route.Handler(w, r)
		}))
		g.mux.HandleFunc(route.Path, authed)
	}
}

func (g *Gateway) backendPackAuth(mode packruntime.BackendRouteAuthMode, next http.HandlerFunc) http.HandlerFunc {
	switch mode {
	case packruntime.BackendRouteAuthPassthrough:
		return next
	default:
		return g.requireAuth(next)
	}
}

func normalizeBackendRouteMethods(route packruntime.BackendRoute) []string {
	seen := map[string]bool{}
	var methods []string
	add := func(method string) {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method == "" || seen[method] {
			return
		}
		seen[method] = true
		methods = append(methods, method)
	}
	add(route.Method)
	for _, method := range route.Methods {
		add(method)
	}
	return methods
}

func backendRouteMethodAllowed(methods []string, requestMethod string) bool {
	requestMethod = strings.ToUpper(strings.TrimSpace(requestMethod))
	if requestMethod == http.MethodHead {
		requestMethod = http.MethodGet
	}
	for _, method := range methods {
		if method == requestMethod {
			return true
		}
	}
	return false
}

func (g *Gateway) backendModuleInfos() []packruntime.BackendModuleInfo {
	g.routesMu.RLock()
	defer g.routesMu.RUnlock()
	byPack := make(map[string][]packruntime.BackendRouteInfo)
	for path, packID := range g.backendPackRoutes {
		if strings.TrimSpace(packID) == "" || strings.TrimSpace(path) == "" {
			continue
		}
		info := g.backendPackRouteInfos[path]
		if strings.TrimSpace(info.Path) == "" {
			info.Path = path
		}
		byPack[packID] = append(byPack[packID], info)
	}
	infos := make([]packruntime.BackendModuleInfo, 0, len(byPack))
	for packID, routes := range byPack {
		slices.SortFunc(routes, func(a, b packruntime.BackendRouteInfo) int { return strings.Compare(a.Path, b.Path) })
		infos = append(infos, packruntime.BackendModuleInfo{PackID: packID, Routes: routes})
	}
	slices.SortFunc(infos, func(a, b packruntime.BackendModuleInfo) int { return strings.Compare(a.PackID, b.PackID) })
	return infos
}

func (g *Gateway) requirePackRoute(packID string, methods []string, route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendRouteMethodAllowed(methods, r.Method) {
			writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		manifestMethod := r.Method
		if manifestMethod == http.MethodHead {
			manifestMethod = http.MethodGet
		}
		if !g.packRouteEnabled(packID, manifestMethod, route) {
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

func (g *Gateway) packRouteEnabled(packID string, method string, route string) bool {
	if g.packRegistry == nil {
		return false
	}
	pack, ok := g.packRegistry.Get(packID)
	if !ok || pack.Status != packruntime.PackStatusEnabled {
		return false
	}
	route = strings.TrimSpace(route)
	return pack.Manifest.Backend.AllowsRoute(method, route)
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

func (g *Gateway) handlePackBackendModules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	modules := g.backendModuleInfos()
	writeJSON(w, map[string]any{"modules": modules, "count": len(modules)})
}

func (g *Gateway) handlePackBackendRouteAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	writeJSON(w, g.backendRouteAuditReport())
}

func (g *Gateway) backendRouteAuditReport() packruntime.BackendRouteAuditReport {
	registry := g.packRegistry
	modules := g.backendModuleInfos()
	report := packruntime.BackendRouteAuditReport{
		GeneratedAt:    time.Now().UTC(),
		MountedModules: len(modules),
		Entries:        []packruntime.BackendRouteAuditEntry{},
	}
	mounted := backendRouteAuditMountedIndex(modules)
	report.MountedRoutes = backendRouteAuditMountedRouteCount(mounted)
	if registry == nil {
		for _, mountedRoute := range backendRouteAuditFlattenMounted(mounted) {
			report.Entries = append(report.Entries, packruntime.BackendRouteAuditEntry{
				PackID:  mountedRoute.packID,
				Status:  "registry-unavailable",
				Mounted: true,
				Method:  mountedRoute.primaryMethod(),
				Methods: append([]string(nil), mountedRoute.methods...),
				Path:    mountedRoute.path,
				Auth:    mountedRoute.auth,
				Issues:  []string{"pack registry is not configured"},
			})
			report.UndeclaredRoutes++
		}
		return report
	}
	packs := registry.List()
	report.Packs = len(packs)
	installedIDs := map[string]bool{}
	for _, pack := range packs {
		installedIDs[pack.Manifest.ID] = true
		if pack.Status == packruntime.PackStatusEnabled {
			report.EnabledPacks++
		}
		declared := backendRouteAuditDeclaredRoutes(pack)
		report.DeclaredRoutes += len(declared)
		seenMountedKeys := map[string]bool{}
		for _, route := range declared {
			entry := packruntime.BackendRouteAuditEntry{
				PackID:      pack.Manifest.ID,
				PackName:    pack.Manifest.Name,
				PackStatus:  string(pack.Status),
				Enabled:     pack.Status == packruntime.PackStatusEnabled,
				Declared:    true,
				Method:      route.method,
				Path:        route.path,
				Description: route.description,
			}
			moduleRoutes := mounted[pack.Manifest.ID+" "+route.path]
			if len(moduleRoutes) == 0 {
				entry.Status = "missing"
				entry.Issues = []string{"manifest route is not mounted by any backend module"}
				report.MissingRoutes++
				report.Entries = append(report.Entries, entry)
				continue
			}
			for _, mountedRoute := range moduleRoutes {
				seenMountedKeys[mountedRoute.key()] = true
				entry.Mounted = true
				entry.Methods = append([]string(nil), mountedRoute.methods...)
				entry.Auth = mountedRoute.auth
				if backendRouteAuditMethodAllowed(mountedRoute.methods, route.method) {
					entry.Status = "ok"
					report.OKRoutes++
				} else {
					entry.Status = "method-mismatch"
					entry.Issues = []string{"manifest routeSpec method is not served by the mounted backend module"}
					report.MethodMismatches++
				}
				report.Entries = append(report.Entries, entry)
			}
		}
		for _, mountedRoute := range backendRouteAuditFlattenMounted(mounted) {
			if mountedRoute.packID != pack.Manifest.ID || seenMountedKeys[mountedRoute.key()] {
				continue
			}
			report.Entries = append(report.Entries, packruntime.BackendRouteAuditEntry{
				PackID:     pack.Manifest.ID,
				PackName:   pack.Manifest.Name,
				PackStatus: string(pack.Status),
				Enabled:    pack.Status == packruntime.PackStatusEnabled,
				Status:     "undeclared",
				Declared:   false,
				Mounted:    true,
				Method:     mountedRoute.primaryMethod(),
				Methods:    append([]string(nil), mountedRoute.methods...),
				Path:       mountedRoute.path,
				Auth:       mountedRoute.auth,
				Issues:     []string{"mounted backend route is not declared by manifest routeSpecs/routes"},
			})
			report.UndeclaredRoutes++
		}
	}
	for _, mountedRoute := range backendRouteAuditFlattenMounted(mounted) {
		if installedIDs[mountedRoute.packID] {
			continue
		}
		report.Entries = append(report.Entries, packruntime.BackendRouteAuditEntry{
			PackID:  mountedRoute.packID,
			Status:  "pack-not-installed",
			Mounted: true,
			Method:  mountedRoute.primaryMethod(),
			Methods: append([]string(nil), mountedRoute.methods...),
			Path:    mountedRoute.path,
			Auth:    mountedRoute.auth,
			Issues:  []string{"backend module is mounted but the pack is not installed in registry"},
		})
		report.UndeclaredRoutes++
	}
	return report
}

type backendRouteAuditDeclaredRoute struct {
	method      string
	path        string
	description string
}

type backendRouteAuditMountedRoute struct {
	packID  string
	path    string
	methods []string
	auth    string
}

func (r backendRouteAuditMountedRoute) key() string {
	return r.packID + " " + r.path + " " + strings.Join(r.methods, ",") + " " + r.auth
}

func (r backendRouteAuditMountedRoute) primaryMethod() string {
	if len(r.methods) == 0 {
		return ""
	}
	return r.methods[0]
}

func backendRouteAuditDeclaredRoutes(pack packruntime.InstalledPack) []backendRouteAuditDeclaredRoute {
	var out []backendRouteAuditDeclaredRoute
	seen := map[string]bool{}
	add := func(method, path, description string) {
		method = strings.ToUpper(strings.TrimSpace(method))
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		key := method + " " + path
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, backendRouteAuditDeclaredRoute{method: method, path: path, description: strings.TrimSpace(description)})
	}
	for _, spec := range pack.Manifest.Backend.RouteSpecs {
		add(spec.Method, spec.Path, spec.Description)
	}
	if len(out) > 0 {
		return out
	}
	for _, path := range pack.Manifest.Backend.Routes {
		add("*", path, "")
	}
	return out
}

func backendRouteAuditMountedIndex(modules []packruntime.BackendModuleInfo) map[string][]backendRouteAuditMountedRoute {
	out := map[string][]backendRouteAuditMountedRoute{}
	for _, module := range modules {
		for _, route := range module.Routes {
			methods := append([]string(nil), route.Methods...)
			if len(methods) == 0 && strings.TrimSpace(route.Method) != "" {
				methods = []string{strings.ToUpper(strings.TrimSpace(route.Method))}
			}
			for i, method := range methods {
				methods[i] = strings.ToUpper(strings.TrimSpace(method))
			}
			path := strings.TrimSpace(route.Path)
			if path == "" {
				continue
			}
			out[module.PackID+" "+path] = append(out[module.PackID+" "+path], backendRouteAuditMountedRoute{
				packID:  module.PackID,
				path:    path,
				methods: methods,
				auth:    route.Auth,
			})
		}
	}
	return out
}

func backendRouteAuditFlattenMounted(index map[string][]backendRouteAuditMountedRoute) []backendRouteAuditMountedRoute {
	var out []backendRouteAuditMountedRoute
	for _, routes := range index {
		out = append(out, routes...)
	}
	slices.SortFunc(out, func(a, b backendRouteAuditMountedRoute) int {
		return strings.Compare(a.key(), b.key())
	})
	return out
}

func backendRouteAuditMountedRouteCount(index map[string][]backendRouteAuditMountedRoute) int {
	total := 0
	for _, routes := range index {
		total += len(routes)
	}
	return total
}

func backendRouteAuditMethodAllowed(methods []string, declaredMethod string) bool {
	declaredMethod = strings.ToUpper(strings.TrimSpace(declaredMethod))
	if declaredMethod == "" || declaredMethod == "*" {
		return len(methods) > 0
	}
	return backendRouteMethodAllowed(methods, declaredMethod)
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
	var artifacts *packruntime.PackArtifacts
	if req.Download {
		artifacts, err = g.packRegistry.CacheDistribution(r.Context(), manifest)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}
	pack, err := g.packRegistry.InstallWithArtifacts(manifest, source, artifacts)
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

func (g *Gateway) handlePackPrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if g.packRegistry == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "pack registry not configured"})
		return
	}
	report := g.packRegistry.PruneArtifacts()
	writeJSON(w, map[string]any{"removed": report.Removed, "kept": report.Kept, "errors": report.Errors, "removed_count": len(report.Removed), "kept_count": len(report.Kept)})
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
