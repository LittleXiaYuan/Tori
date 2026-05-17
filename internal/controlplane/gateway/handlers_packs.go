package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	g.mux.HandleFunc("/v1/packs/catalog", g.requireAuth(g.handlePackCatalog))
	g.mux.HandleFunc("/v1/packs/installed", g.requireAuth(g.handlePacksList))
	g.mux.HandleFunc("/v1/packs/enabled", g.requireAuth(g.handlePacksEnabled))
	g.mux.HandleFunc("/v1/packs/capabilities/plan", g.requireAuth(g.handlePackCapabilityPlan))
	g.mux.HandleFunc("/v1/packs/capabilities/prepare", g.requireAuth(g.handlePackCapabilityPrepare))
	g.mux.HandleFunc("/v1/packs/capabilities/gate", g.requireAuth(g.handlePackCapabilityGate))
	g.mux.HandleFunc("/v1/packs/capabilities/resolve", g.requireAuth(g.handlePackCapabilityResolve))
	g.mux.HandleFunc("/v1/packs/capabilities", g.requireAuth(g.handlePackCapabilities))
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

func (g *Gateway) handlePackCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	writeJSON(w, g.packCatalogReport(r.URL.Query().Get("capability"), r.URL.Query().Get("q")))
}

func (g *Gateway) packCatalogReport(capability string, query string) packruntime.PackCatalogReport {
	capability = strings.TrimSpace(capability)
	query = strings.ToLower(strings.TrimSpace(query))
	sources := g.packCatalogSourceDirs()
	report := packruntime.PackCatalogReport{
		GeneratedAt:   time.Now().UTC(),
		Sources:       append([]string(nil), sources...),
		Capability:    capability,
		Query:         query,
		Entries:       []packruntime.PackCatalogEntry{},
		SourceReports: []packruntime.PackCatalogSourceReport{},
	}
	installed := map[string]packruntime.InstalledPack{}
	if g.packRegistry != nil {
		for _, pack := range g.packRegistry.List() {
			installed[pack.Manifest.ID] = pack
		}
	}
	seen := map[string]bool{}
	for _, source := range sources {
		sourceReport := packruntime.PackCatalogSourceReport{Source: source, OK: true}
		manifestPaths, err := packCatalogManifestPaths(source)
		if err != nil {
			message := err.Error()
			sourceReport.OK = false
			sourceReport.Errors = append(sourceReport.Errors, message)
			report.Errors = append(report.Errors, message)
			report.SourceReports = append(report.SourceReports, sourceReport)
			continue
		}
		sourceReport.ManifestCount = len(manifestPaths)
		for _, manifestPath := range manifestPaths {
			manifest, err := packruntime.LoadManifest(manifestPath)
			if err != nil {
				message := fmt.Sprintf("%s: %v", manifestPath, err)
				sourceReport.OK = false
				sourceReport.Errors = append(sourceReport.Errors, message)
				report.Errors = append(report.Errors, message)
				continue
			}
			if seen[manifest.ID] {
				continue
			}
			seen[manifest.ID] = true
			entry := packCatalogEntry(manifestPath, source, manifest, installed[manifest.ID])
			if capability != "" && !manifestProvidesCapability(manifest, capability) {
				continue
			}
			if query != "" && !packCatalogEntryMatches(entry, query) {
				continue
			}
			report.Entries = append(report.Entries, entry)
			sourceReport.MatchedEntries++
		}
		report.SourceReports = append(report.SourceReports, sourceReport)
	}
	slices.SortFunc(report.Entries, func(a, b packruntime.PackCatalogEntry) int {
		return strings.Compare(a.Manifest.ID, b.Manifest.ID)
	})
	capabilitySet := map[string]bool{}
	for _, entry := range report.Entries {
		report.Count++
		if entry.Installed {
			report.Installed++
		}
		if entry.Enabled {
			report.Enabled++
		}
		if entry.Downloadable {
			report.Downloadable++
			report.DownloadHints = append(report.DownloadHints, entry)
		}
		for _, cap := range entry.Manifest.Backend.Capabilities {
			if strings.TrimSpace(cap) != "" {
				capabilitySet[cap] = true
			}
		}
		switch entry.UpdateAction {
		case "install":
			report.InstallHints = append(report.InstallHints, entry)
		case "enable":
			report.EnableHints = append(report.EnableHints, entry)
		}
	}
	report.Capabilities = len(capabilitySet)
	return report
}

func (g *Gateway) packCatalogSourceDirs() []string {
	if len(g.packCatalogSources) > 0 {
		return append([]string(nil), g.packCatalogSources...)
	}
	return []string{filepath.Join("packs", "examples"), filepath.Join("packs", "templates")}
}

func packCatalogManifestPaths(source string) ([]string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, nil
	}
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("pack catalog source %s: %w", source, err)
	}
	if !info.IsDir() {
		if filepath.Base(source) == packruntime.ManifestFileName {
			return []string{source}, nil
		}
		return nil, fmt.Errorf("pack catalog source %s is not a directory or %s", source, packruntime.ManifestFileName)
	}
	var paths []string
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Base(path) == packruntime.ManifestFileName {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk pack catalog source %s: %w", source, err)
	}
	slices.Sort(paths)
	return paths, nil
}

func packCatalogEntry(manifestPath string, source string, manifest packruntime.Manifest, installed packruntime.InstalledPack) packruntime.PackCatalogEntry {
	entry := packruntime.PackCatalogEntry{
		ManifestPath: manifestPath,
		Source:       source,
		Manifest:     manifest,
		UpdateAction: "install",
		Downloadable: strings.TrimSpace(manifest.Distribution.PackageURL) != "" && strings.TrimSpace(manifest.Distribution.SHA256) != "",
	}
	if installed.Manifest.ID == manifest.ID {
		entry.Installed = true
		entry.Status = installed.Status
		entry.Enabled = installed.Status == packruntime.PackStatusEnabled
		if installed.Manifest.Version != manifest.Version {
			entry.UpdateAction = "update"
		} else if installed.Status == packruntime.PackStatusDisabled {
			entry.UpdateAction = "enable"
		} else {
			entry.UpdateAction = "use"
		}
	}
	return entry
}

func manifestProvidesCapability(manifest packruntime.Manifest, capability string) bool {
	for _, candidate := range manifest.Backend.Capabilities {
		if strings.TrimSpace(candidate) == capability {
			return true
		}
	}
	return false
}

func packCatalogEntryMatches(entry packruntime.PackCatalogEntry, query string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		entry.Manifest.ID,
		entry.Manifest.Name,
		entry.Manifest.Description,
		entry.Manifest.SDK.TypeScript,
		strings.Join(entry.Manifest.Backend.Capabilities, " "),
		strings.Join(entry.Manifest.Backend.Permissions, " "),
		entry.Manifest.Metadata["blueprint"],
		entry.Manifest.Metadata["stage"],
	}, " "))
	return strings.Contains(haystack, query)
}

func (g *Gateway) handlePackBackendModules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	modules := g.backendModuleInfos()
	writeJSON(w, map[string]any{"modules": modules, "count": len(modules)})
}

func (g *Gateway) handlePackCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	writeJSON(w, g.packCapabilityIndexReport())
}

func (g *Gateway) handlePackCapabilityResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	capability := strings.TrimSpace(r.URL.Query().Get("capability"))
	if capability == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "capability is required"})
		return
	}
	writeJSON(w, g.packCapabilityResolveReport(capability))
}

func (g *Gateway) handlePackCapabilityGate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	capability := strings.TrimSpace(r.URL.Query().Get("capability"))
	if capability == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "capability is required"})
		return
	}
	writeJSON(w, g.packCapabilityGateReport(capability))
}

func (g *Gateway) handlePackCapabilityPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	capabilities := parseCapabilityPlanQuery(r.URL.Query()["capability"])
	if len(capabilities) == 0 {
		capabilities = parseCapabilityPlanQuery([]string{r.URL.Query().Get("capabilities")})
	}
	if len(capabilities) == 0 {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "capability or capabilities is required"})
		return
	}
	writeJSON(w, g.packCapabilityPlanReport(capabilities))
}

func (g *Gateway) handlePackCapabilityPrepare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	capabilities := parseCapabilityPlanQuery(r.URL.Query()["capability"])
	if len(capabilities) == 0 {
		capabilities = parseCapabilityPlanQuery([]string{r.URL.Query().Get("capabilities")})
	}
	if len(capabilities) == 0 {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "capability or capabilities is required"})
		return
	}
	writeJSON(w, g.packCapabilityPrepareReport(capabilities))
}

func (g *Gateway) packCapabilityIndexReport() packruntime.CapabilityIndexReport {
	report := packruntime.CapabilityIndexReport{
		GeneratedAt: time.Now().UTC(),
		Entries:     []packruntime.CapabilityIndexEntry{},
	}
	if g.packRegistry == nil {
		return report
	}
	packs := g.packRegistry.List()
	report.Packs = len(packs)
	for _, pack := range packs {
		enabled := pack.Status == packruntime.PackStatusEnabled
		if enabled {
			report.EnabledPacks++
		}
		for _, capability := range pack.Manifest.Backend.Capabilities {
			capability = strings.TrimSpace(capability)
			if capability == "" {
				continue
			}
			entry := packruntime.CapabilityIndexEntry{
				Capability:    capability,
				PackID:        pack.Manifest.ID,
				PackName:      pack.Manifest.Name,
				PackStatus:    string(pack.Status),
				Enabled:       enabled,
				Optional:      pack.Manifest.Optional,
				Routes:        append([]string(nil), pack.Manifest.Backend.Routes...),
				Permissions:   append([]string(nil), pack.Manifest.Backend.Permissions...),
				SDKTypeScript: pack.Manifest.SDK.TypeScript,
				FrontendPaths: packCapabilityFrontendPaths(pack.Manifest),
			}
			report.Entries = append(report.Entries, entry)
			report.Capabilities++
			if enabled {
				report.EnabledCapabilities++
			}
		}
	}
	slices.SortFunc(report.Entries, func(a, b packruntime.CapabilityIndexEntry) int {
		if cmp := strings.Compare(a.Capability, b.Capability); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.PackID, b.PackID)
	})
	return report
}

func (g *Gateway) packCapabilityPlanReport(capabilities []string) packruntime.CapabilityPlanReport {
	report := packruntime.CapabilityPlanReport{
		GeneratedAt:           time.Now().UTC(),
		Capabilities:          normalizeCapabilityPlanInputs(capabilities),
		Action:                "use",
		Gates:                 []packruntime.CapabilityGateReport{},
		RequiredPacks:         []packruntime.CapabilityIndexEntry{},
		EnablePacks:           []packruntime.CapabilityIndexEntry{},
		InstallCapabilities:   []string{},
		CatalogInstallHints:   []packruntime.PackCatalogEntry{},
		CatalogDownloadHints:  []packruntime.PackCatalogEntry{},
		CatalogSourceReports:  []packruntime.PackCatalogSourceReport{},
		RouteAuditIssues:      []packruntime.BackendRouteAuditEntry{},
		UnavailableReasons:    []string{},
		DownloadablePackHints: []packruntime.CapabilityIndexEntry{},
	}
	seenRequired := map[string]bool{}
	seenEnable := map[string]bool{}
	seenHints := map[string]bool{}
	seenCatalogInstall := map[string]bool{}
	seenCatalogDownload := map[string]bool{}
	seenCatalogSources := map[string]int{}
	seenInstall := map[string]bool{}
	seenReasons := map[string]bool{}
	for _, capability := range report.Capabilities {
		gate := g.packCapabilityGateReport(capability)
		report.Gates = append(report.Gates, gate)
		if gate.Allowed {
			report.AllowedCount++
		} else {
			report.BlockedCount++
			if gate.Reason != "" && !seenReasons[gate.Capability+" "+gate.Reason] {
				report.UnavailableReasons = append(report.UnavailableReasons, gate.Reason)
				seenReasons[gate.Capability+" "+gate.Reason] = true
			}
		}
		switch gate.Action {
		case "use":
			report.UseCount++
			if gate.Allowed && gate.Resolution.Preferred != nil {
				addCapabilityPlanEntry(&report.RequiredPacks, seenRequired, *gate.Resolution.Preferred)
			}
		case "enable":
			report.EnableCount++
			if gate.Resolution.Preferred != nil {
				addCapabilityPlanEntry(&report.EnablePacks, seenEnable, *gate.Resolution.Preferred)
				addCapabilityPlanEntry(&report.DownloadablePackHints, seenHints, *gate.Resolution.Preferred)
			}
		default:
			report.InstallCount++
			if !seenInstall[gate.Capability] {
				report.InstallCapabilities = append(report.InstallCapabilities, gate.Capability)
				seenInstall[gate.Capability] = true
			}
			catalog := g.packCatalogReport(gate.Capability, "")
			addCapabilityPlanCatalogSourceReports(&report.CatalogSourceReports, seenCatalogSources, catalog.SourceReports)
			for _, hint := range catalog.InstallHints {
				addCapabilityPlanCatalogEntry(&report.CatalogInstallHints, seenCatalogInstall, hint)
				if hint.Downloadable {
					addCapabilityPlanCatalogEntry(&report.CatalogDownloadHints, seenCatalogDownload, hint)
				}
			}
			for _, hint := range catalog.DownloadHints {
				addCapabilityPlanCatalogEntry(&report.CatalogDownloadHints, seenCatalogDownload, hint)
			}
		}
		for _, audit := range gate.RouteAudit {
			if audit.Status != "ok" {
				report.RouteAuditIssues = append(report.RouteAuditIssues, audit)
				report.RouteAuditIssueCount++
			}
		}
	}
	report.Allowed = report.BlockedCount == 0 && len(report.Capabilities) > 0
	switch {
	case report.InstallCount > 0:
		report.Action = "install"
	case report.EnableCount > 0:
		report.Action = "enable"
	case report.RouteAuditIssueCount > 0:
		report.Action = "fix-route-audit"
	default:
		report.Action = "use"
	}
	slices.Sort(report.InstallCapabilities)
	slices.SortFunc(report.CatalogInstallHints, func(a, b packruntime.PackCatalogEntry) int {
		return strings.Compare(a.Manifest.ID, b.Manifest.ID)
	})
	slices.SortFunc(report.CatalogDownloadHints, func(a, b packruntime.PackCatalogEntry) int {
		return strings.Compare(a.Manifest.ID, b.Manifest.ID)
	})
	slices.SortFunc(report.RequiredPacks, func(a, b packruntime.CapabilityIndexEntry) int {
		return strings.Compare(a.PackID+" "+a.Capability, b.PackID+" "+b.Capability)
	})
	slices.SortFunc(report.EnablePacks, func(a, b packruntime.CapabilityIndexEntry) int {
		return strings.Compare(a.PackID+" "+a.Capability, b.PackID+" "+b.Capability)
	})
	slices.SortFunc(report.DownloadablePackHints, func(a, b packruntime.CapabilityIndexEntry) int {
		return strings.Compare(a.PackID+" "+a.Capability, b.PackID+" "+b.Capability)
	})
	return report
}

func (g *Gateway) packCapabilityPrepareReport(capabilities []string) packruntime.CapabilityPrepareReport {
	plan := g.packCapabilityPlanReport(capabilities)
	report := packruntime.CapabilityPrepareReport{
		GeneratedAt:          time.Now().UTC(),
		Capabilities:         append([]string(nil), plan.Capabilities...),
		Allowed:              plan.Allowed,
		Action:               plan.Action,
		Plan:                 plan,
		Steps:                []packruntime.CapabilityPrepareStep{},
		CatalogSourceReports: append([]packruntime.PackCatalogSourceReport(nil), plan.CatalogSourceReports...),
		UnavailableReasons:   append([]string(nil), plan.UnavailableReasons...),
		RouteAuditIssues:     append([]packruntime.BackendRouteAuditEntry(nil), plan.RouteAuditIssues...),
		RouteAuditIssueCount: plan.RouteAuditIssueCount,
	}
	seen := map[string]bool{}
	for _, entry := range plan.RequiredPacks {
		addCapabilityPrepareStep(&report, seen, packCapabilityPrepareUseStep(entry))
	}
	for _, entry := range plan.EnablePacks {
		addCapabilityPrepareStep(&report, seen, packCapabilityPrepareEnableStep(entry))
	}
	for _, entry := range plan.CatalogInstallHints {
		step := packCapabilityPrepareInstallStep(entry)
		addCapabilityPrepareStep(&report, seen, step)
		if entry.Downloadable {
			downloadStep := step
			downloadStep.Action = "download"
			downloadStep.Reason = "download and verify this package before installing the pack manifest"
			addCapabilityPrepareStep(&report, seen, downloadStep)
		}
	}
	for _, issue := range plan.RouteAuditIssues {
		step := packruntime.CapabilityPrepareStep{
			Action:       "fix-route-audit",
			PackID:       issue.PackID,
			PackName:     issue.PackName,
			Enabled:      issue.Enabled,
			Downloadable: false,
			Reason:       strings.Join(issue.Issues, "; "),
		}
		if step.Reason == "" {
			step.Reason = fmt.Sprintf("backend route %s %s is %s", issue.Method, issue.Path, issue.Status)
		}
		addCapabilityPrepareStep(&report, seen, step)
	}
	for _, capability := range plan.InstallCapabilities {
		if hasPrepareInstallStepForCapability(report.InstallSteps, capability) {
			continue
		}
		addCapabilityPrepareStep(&report, seen, packruntime.CapabilityPrepareStep{
			Action:     "install",
			Capability: capability,
			Reason:     packCapabilityCatalogSourceReason("no installed or catalog pack currently provides this capability", report.CatalogSourceReports),
		})
	}
	report.StepCount = len(report.Steps)
	report.Allowed = plan.Allowed && report.InstallCount == 0 && report.EnableCount == 0 && report.RouteAuditIssueCount == 0
	switch {
	case report.RouteAuditIssueCount > 0:
		report.Action = "fix-route-audit"
	case report.InstallCount > 0:
		report.Action = "install"
	case report.EnableCount > 0:
		report.Action = "enable"
	default:
		report.Action = "use"
	}
	return report
}

func packCapabilityPrepareUseStep(entry packruntime.CapabilityIndexEntry) packruntime.CapabilityPrepareStep {
	entryCopy := entry
	return packruntime.CapabilityPrepareStep{
		Action:         "use",
		PackID:         entry.PackID,
		PackName:       entry.PackName,
		Capability:     entry.Capability,
		Installed:      true,
		Enabled:        entry.Enabled,
		Reason:         "capability is already available through an enabled pack",
		CapabilityInfo: &entryCopy,
	}
}

func packCapabilityPrepareEnableStep(entry packruntime.CapabilityIndexEntry) packruntime.CapabilityPrepareStep {
	entryCopy := entry
	return packruntime.CapabilityPrepareStep{
		Action:         "enable",
		PackID:         entry.PackID,
		PackName:       entry.PackName,
		Capability:     entry.Capability,
		Installed:      true,
		Enabled:        entry.Enabled,
		Reason:         "capability is installed but the pack is disabled",
		CapabilityInfo: &entryCopy,
	}
}

func packCapabilityPrepareInstallStep(entry packruntime.PackCatalogEntry) packruntime.CapabilityPrepareStep {
	entryCopy := entry
	capability := ""
	if len(entry.Manifest.Backend.Capabilities) > 0 {
		capability = entry.Manifest.Backend.Capabilities[0]
	}
	return packruntime.CapabilityPrepareStep{
		Action:       "install",
		PackID:       entry.Manifest.ID,
		PackName:     entry.Manifest.Name,
		Capability:   capability,
		ManifestPath: entry.ManifestPath,
		ManifestURL:  entry.Manifest.Distribution.ManifestURL,
		PackageURL:   entry.Manifest.Distribution.PackageURL,
		FrontendURL:  entry.Manifest.Distribution.FrontendURL,
		SHA256:       entry.Manifest.Distribution.SHA256,
		SizeBytes:    entry.Manifest.Distribution.SizeBytes,
		Installed:    entry.Installed,
		Enabled:      entry.Enabled,
		Downloadable: entry.Downloadable,
		Reason:       "install this catalog pack to provide one or more missing capabilities",
		CatalogEntry: &entryCopy,
	}
}

func addCapabilityPrepareStep(report *packruntime.CapabilityPrepareReport, seen map[string]bool, step packruntime.CapabilityPrepareStep) {
	key := strings.Join([]string{step.Action, step.PackID, step.Capability, step.ManifestPath, step.PackageURL}, "\x00")
	if seen[key] {
		return
	}
	seen[key] = true
	report.Steps = append(report.Steps, step)
	switch step.Action {
	case "use":
		report.ReadyCount++
		report.UseSteps = append(report.UseSteps, step)
	case "enable":
		report.EnableCount++
		report.EnableSteps = append(report.EnableSteps, step)
	case "download":
		report.DownloadCount++
		report.DownloadSteps = append(report.DownloadSteps, step)
	case "fix-route-audit":
		report.RouteAuditFixSteps = append(report.RouteAuditFixSteps, step)
	default:
		report.InstallCount++
		report.InstallSteps = append(report.InstallSteps, step)
	}
}

func hasPrepareInstallStepForCapability(steps []packruntime.CapabilityPrepareStep, capability string) bool {
	for _, step := range steps {
		if step.Capability == capability || (step.CatalogEntry != nil && manifestProvidesCapability(step.CatalogEntry.Manifest, capability)) {
			return true
		}
	}
	return false
}

func packCapabilityCatalogSourceReason(base string, reports []packruntime.PackCatalogSourceReport) string {
	parts := []string{}
	for _, report := range reports {
		source := strings.TrimSpace(report.Source)
		if source == "" {
			continue
		}
		status := "ok"
		if !report.OK {
			status = "error"
		}
		part := fmt.Sprintf("%s %s manifests=%d matched=%d", source, status, report.ManifestCount, report.MatchedEntries)
		if len(report.Errors) > 0 {
			part += " errors=" + strings.Join(report.Errors, " | ")
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return base
	}
	return base + "; catalog sources scanned: " + strings.Join(parts, "; ")
}

func (g *Gateway) packCapabilityGateReport(capability string) packruntime.CapabilityGateReport {
	resolution := g.packCapabilityResolveReport(capability)
	report := packruntime.CapabilityGateReport{
		GeneratedAt: time.Now().UTC(),
		Capability:  resolution.Capability,
		Action:      resolution.Action,
		Resolution:  resolution,
	}
	if !resolution.Found {
		report.Reason = "capability is not provided by any installed pack"
		return report
	}
	if !resolution.Enabled {
		report.Reason = "capability is provided only by disabled packs"
		return report
	}
	auditEntries := g.backendRouteAuditReport().Entries
	for _, entry := range auditEntries {
		if resolution.Preferred == nil || entry.PackID != resolution.Preferred.PackID {
			continue
		}
		for _, route := range resolution.Preferred.Routes {
			if entry.Path == route {
				report.RouteAudit = append(report.RouteAudit, entry)
			}
		}
	}
	for _, entry := range report.RouteAudit {
		if entry.Status != "ok" {
			report.Reason = "capability pack has backend route audit issues"
			return report
		}
	}
	report.Allowed = true
	report.Reason = "capability is available through an enabled pack"
	return report
}

func (g *Gateway) packCapabilityResolveReport(capability string) packruntime.CapabilityResolveReport {
	capability = strings.TrimSpace(capability)
	report := packruntime.CapabilityResolveReport{
		GeneratedAt:    time.Now().UTC(),
		Capability:     capability,
		Action:         "install",
		Entries:        []packruntime.CapabilityIndexEntry{},
		EnabledEntries: []packruntime.CapabilityIndexEntry{},
	}
	if capability == "" {
		return report
	}
	index := g.packCapabilityIndexReport()
	for _, entry := range index.Entries {
		if entry.Capability != capability {
			continue
		}
		report.Found = true
		report.Entries = append(report.Entries, entry)
		if entry.Enabled {
			report.Enabled = true
			report.EnabledEntries = append(report.EnabledEntries, entry)
		}
	}
	if len(report.EnabledEntries) > 0 {
		preferred := report.EnabledEntries[0]
		report.Preferred = &preferred
		report.Action = "use"
		return report
	}
	if len(report.Entries) > 0 {
		preferred := report.Entries[0]
		report.Preferred = &preferred
		report.Action = "enable"
	}
	return report
}

func parseCapabilityPlanQuery(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func normalizeCapabilityPlanInputs(capabilities []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, capability := range capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" || seen[capability] {
			continue
		}
		seen[capability] = true
		out = append(out, capability)
	}
	slices.Sort(out)
	return out
}

func addCapabilityPlanEntry(entries *[]packruntime.CapabilityIndexEntry, seen map[string]bool, entry packruntime.CapabilityIndexEntry) {
	key := entry.PackID + " " + entry.Capability
	if seen[key] {
		return
	}
	seen[key] = true
	*entries = append(*entries, entry)
}

func addCapabilityPlanCatalogEntry(entries *[]packruntime.PackCatalogEntry, seen map[string]bool, entry packruntime.PackCatalogEntry) {
	key := entry.Manifest.ID + " " + entry.Manifest.Version
	if seen[key] {
		return
	}
	seen[key] = true
	*entries = append(*entries, entry)
}

func addCapabilityPlanCatalogSourceReports(reports *[]packruntime.PackCatalogSourceReport, seen map[string]int, incoming []packruntime.PackCatalogSourceReport) {
	for _, sourceReport := range incoming {
		if sourceReport.Source == "" {
			continue
		}
		if index, ok := seen[sourceReport.Source]; ok {
			existing := &(*reports)[index]
			existing.ManifestCount = max(existing.ManifestCount, sourceReport.ManifestCount)
			existing.MatchedEntries += sourceReport.MatchedEntries
			existing.OK = existing.OK && sourceReport.OK
			for _, message := range sourceReport.Errors {
				if message == "" || slices.Contains(existing.Errors, message) {
					continue
				}
				existing.Errors = append(existing.Errors, message)
			}
			continue
		}
		copyReport := sourceReport
		copyReport.Errors = append([]string(nil), sourceReport.Errors...)
		seen[sourceReport.Source] = len(*reports)
		*reports = append(*reports, copyReport)
	}
}

func packCapabilityFrontendPaths(manifest packruntime.Manifest) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, menu := range manifest.Frontend.Menus {
		add(menu.Path)
	}
	for _, route := range manifest.Frontend.Routes {
		add(route.Path)
	}
	slices.Sort(paths)
	return paths
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
