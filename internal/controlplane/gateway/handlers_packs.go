package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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
	PackageURL   string `json:"package_url"`
	SHA256       string `json:"sha256"`
	Source       string `json:"source"`
	Download     bool   `json:"download"`
}

type packReleaseCatalogRequest struct {
	Releases []string `json:"releases"`
}

type packReleaseCatalogEntry struct {
	ReleaseURL   string                 `json:"release_url"`
	ReleaseTag   string                 `json:"release_tag,omitempty"`
	ReleaseName  string                 `json:"release_name,omitempty"`
	PublishedAt  string                 `json:"published_at,omitempty"`
	PackageURL   string                 `json:"package_url"`
	AssetName    string                 `json:"asset_name,omitempty"`
	SHA256       string                 `json:"sha256,omitempty"`
	SizeBytes    int64                  `json:"size_bytes,omitempty"`
	Manifest     packruntime.Manifest   `json:"manifest"`
	Installed    bool                   `json:"installed"`
	Enabled      bool                   `json:"enabled"`
	Status       packruntime.PackStatus `json:"status,omitempty"`
	UpdateAction string                 `json:"update_action"`
	Downloadable bool                   `json:"downloadable"`
}

type packReleaseCatalogReport struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	Releases    []string                  `json:"releases"`
	Count       int                       `json:"count"`
	Entries     []packReleaseCatalogEntry `json:"entries"`
	Errors      []string                  `json:"errors,omitempty"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

type githubReleaseResponse struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	HTMLURL     string               `json:"html_url"`
	PublishedAt string               `json:"published_at"`
	Assets      []githubReleaseAsset `json:"assets"`
}

var githubAssetHrefPattern = regexp.MustCompile(`href="([^"]+\.yqpack[^"]*)"`)

func (g *Gateway) registerPackRoutes() {
	g.mux.HandleFunc("/v1/packs", g.requireAuth(g.handlePacksList))
	g.mux.HandleFunc("/v1/packs/catalog", g.requireAuth(g.handlePackCatalog))
	g.mux.HandleFunc("/v1/packs/release-catalog", g.requireAuth(g.handlePackReleaseCatalog))
	g.mux.HandleFunc("/v1/packs/installed", g.requireAuth(g.handlePacksList))
	g.mux.HandleFunc("/v1/packs/enabled", g.requireAuth(g.handlePacksEnabled))
	g.mux.HandleFunc("/v1/packs/capabilities/plan", g.requireAuth(g.handlePackCapabilityPlan))
	g.mux.HandleFunc("/v1/packs/capabilities/prepare", g.requireAuth(g.handlePackCapabilityPrepare))
	g.mux.HandleFunc("/v1/packs/capabilities/gate", g.requireAuth(g.handlePackCapabilityGate))
	g.mux.HandleFunc("/v1/packs/capabilities/resolve", g.requireAuth(g.handlePackCapabilityResolve))
	g.mux.HandleFunc("/v1/packs/capabilities/cogni-profile", g.requireAuth(g.handlePackCapabilityCogniProfile))
	g.mux.HandleFunc("/v1/packs/capabilities", g.requireAuth(g.handlePackCapabilities))
	g.mux.HandleFunc("/v1/packs/backend-modules", g.requireAuth(g.handlePackBackendModules))
	g.mux.HandleFunc("/v1/packs/backend-route-audit", g.requireAuth(g.handlePackBackendRouteAudit))
	g.mux.HandleFunc("/v1/packs/studio/plan", g.requireAuth(g.handlePackStudioPlan))
	g.mux.HandleFunc("/v1/packs/studio/inspect", g.requireAuth(g.handlePackStudioInspect))
	g.mux.HandleFunc("/v1/packs/studio/workspace", g.requireAuth(g.handlePackStudioWorkspace))
	g.mux.HandleFunc("/v1/packs/studio/patch", g.requireAuth(g.handlePackStudioPatch))
	g.mux.HandleFunc("/v1/packs/install", g.requireAuth(g.handlePackInstall))
	g.mux.HandleFunc("/v1/packs/enable", g.requireAuth(g.handlePackEnable))
	g.mux.HandleFunc("/v1/packs/disable", g.requireAuth(g.handlePackDisable))
	g.mux.HandleFunc("/v1/packs/rollback", g.requireAuth(g.handlePackRollback))
	g.mux.HandleFunc("/v1/packs/prune", g.requireAuth(g.handlePackPrune))
	// Pack UI bundles (DLC iframe host). Public static assets — see
	// handlePackUIAsset; privileged actions go through the authed bridge. The
	// bare-root form is registered separately because stripTrailingSlash rewrites
	// "/ui/" → "/ui", which the subtree pattern would otherwise 307-redirect.
	g.mux.HandleFunc("GET /v1/packs/{id}/ui", g.handlePackUIAsset)
	g.mux.HandleFunc("GET /v1/packs/{id}/ui/{path...}", g.handlePackUIAsset)
	g.mux.HandleFunc("/v1/packs/ui-origin", g.requireAuth(g.handlePackUIOrigin))
	// Bridge-violation audit sink (spec §7.3) — reported by the authed shell.
	g.mux.HandleFunc("POST /v1/packs/{id}/bridge-violation", g.requireAuth(g.handlePackBridgeViolation))
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
	return []string{filepath.Join("packs", "official"), filepath.Join("packs", "templates")}
}

func (g *Gateway) handlePackReleaseCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req packReleaseCatalogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if len(req.Releases) == 0 {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "releases is required"})
		return
	}
	writeJSON(w, g.packReleaseCatalogReport(r, req.Releases))
}

func (g *Gateway) packReleaseCatalogReport(r *http.Request, releaseURLs []string) packReleaseCatalogReport {
	report := packReleaseCatalogReport{
		GeneratedAt: time.Now().UTC(),
		Releases:    []string{},
		Entries:     []packReleaseCatalogEntry{},
	}
	installed := map[string]packruntime.InstalledPack{}
	if g.packRegistry != nil {
		for _, pack := range g.packRegistry.List() {
			installed[pack.Manifest.ID] = pack
		}
	}
	seenReleases := map[string]bool{}
	seenPackages := map[string]bool{}
	for _, releaseURL := range releaseURLs {
		releaseURL = strings.TrimSpace(releaseURL)
		if releaseURL == "" || seenReleases[releaseURL] {
			continue
		}
		seenReleases[releaseURL] = true
		report.Releases = append(report.Releases, releaseURL)
		release, err := fetchGitHubRelease(r, releaseURL)
		if err != nil {
			fallbackRelease, fallbackErr := scrapeGitHubReleaseAssets(r, releaseURL)
			if fallbackErr != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", releaseURL, err))
				report.Errors = append(report.Errors, fmt.Sprintf("%s: fallback scrape: %v", releaseURL, fallbackErr))
				continue
			}
			release = fallbackRelease
		}
		entryReleaseURL := release.HTMLURL
		if strings.TrimSpace(entryReleaseURL) == "" {
			entryReleaseURL = releaseURL
		}
		for _, asset := range release.Assets {
			packageURL := strings.TrimSpace(asset.BrowserDownloadURL)
			if packageURL == "" || !strings.HasSuffix(strings.ToLower(strings.TrimSpace(asset.Name)), ".yqpack") || seenPackages[packageURL] {
				continue
			}
			seenPackages[packageURL] = true
			manifest, artifactSHA, err := fetchYqpackManifest(r, packageURL)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", packageURL, err))
				continue
			}
			if manifest.Distribution.PackageURL == "" {
				manifest.Distribution.PackageURL = packageURL
			}
			assetSHA := strings.TrimSpace(asset.Digest)
			if assetSHA == "" {
				assetSHA = artifactSHA
			}
			if manifest.Distribution.SHA256 == "" {
				manifest.Distribution.SHA256 = assetSHA
			}
			if manifest.Distribution.SizeBytes == 0 && asset.Size > 0 {
				manifest.Distribution.SizeBytes = asset.Size
			}
			if manifest.Distribution.ManifestURL == "" {
				manifest.Distribution.ManifestURL = entryReleaseURL
			}
			entry := packReleaseCatalogEntry{
				ReleaseURL:   entryReleaseURL,
				ReleaseTag:   release.TagName,
				ReleaseName:  release.Name,
				PublishedAt:  release.PublishedAt,
				PackageURL:   packageURL,
				AssetName:    asset.Name,
				SHA256:       assetSHA,
				SizeBytes:    asset.Size,
				Manifest:     manifest,
				UpdateAction: "install",
				Downloadable: true,
			}
			if installedPack, ok := installed[manifest.ID]; ok {
				entry.Installed = true
				entry.Status = installedPack.Status
				entry.Enabled = installedPack.Status == packruntime.PackStatusEnabled
				if installedPack.Manifest.Version != manifest.Version {
					entry.UpdateAction = "update"
				} else if installedPack.Status == packruntime.PackStatusDisabled {
					entry.UpdateAction = "enable"
				} else {
					entry.UpdateAction = "use"
				}
			}
			report.Entries = append(report.Entries, entry)
		}
	}
	slices.SortFunc(report.Entries, func(a, b packReleaseCatalogEntry) int {
		if cmp := strings.Compare(a.Manifest.ID, b.Manifest.ID); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.ReleaseTag, b.ReleaseTag)
	})
	report.Count = len(report.Entries)
	return report
}

func fetchGitHubRelease(r *http.Request, releaseURL string) (githubReleaseResponse, error) {
	apiURL, err := githubReleaseAPIURL(releaseURL)
	if err != nil {
		return githubReleaseResponse{}, err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, nil)
	if err != nil {
		return githubReleaseResponse{}, fmt.Errorf("create github release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Yunque-PackRuntime")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubReleaseResponse{}, fmt.Errorf("download github release: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return githubReleaseResponse{}, fmt.Errorf("download github release: http %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return githubReleaseResponse{}, fmt.Errorf("read github release: %w", err)
	}
	var release githubReleaseResponse
	if err := json.Unmarshal(data, &release); err != nil {
		return githubReleaseResponse{}, fmt.Errorf("parse github release: %w", err)
	}
	return release, nil
}

func scrapeGitHubReleaseAssets(r *http.Request, releaseURL string) (githubReleaseResponse, error) {
	parsed, err := parseGitHubReleaseURL(releaseURL)
	if err != nil {
		return githubReleaseResponse{}, err
	}
	expandedURL := fmt.Sprintf("https://github.com/%s/%s/releases/expanded_assets/%s", url.PathEscape(parsed.owner), url.PathEscape(parsed.repo), strings.ReplaceAll(url.PathEscape(parsed.tag), "%2F", "/"))
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, expandedURL, nil)
	if err != nil {
		return githubReleaseResponse{}, fmt.Errorf("create github assets request: %w", err)
	}
	req.Header.Set("Accept", "text/fragment+html")
	req.Header.Set("User-Agent", "Yunque-PackRuntime")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubReleaseResponse{}, fmt.Errorf("download github assets: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return githubReleaseResponse{}, fmt.Errorf("download github assets: http %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return githubReleaseResponse{}, fmt.Errorf("read github assets: %w", err)
	}
	fragment := string(data)
	var assets []githubReleaseAsset
	seen := map[string]bool{}
	for _, match := range githubAssetHrefPattern.FindAllStringSubmatch(fragment, -1) {
		if len(match) < 2 {
			continue
		}
		href := strings.TrimSpace(htmlUnescape(match[1]))
		if href == "" {
			continue
		}
		assetURL, err := url.Parse(href)
		if err != nil {
			continue
		}
		if !assetURL.IsAbs() {
			assetURL = (&url.URL{Scheme: "https", Host: "github.com"}).ResolveReference(assetURL)
		}
		downloadURL := assetURL.String()
		if seen[downloadURL] {
			continue
		}
		seen[downloadURL] = true
		name, _ := url.PathUnescape(pathfileBase(assetURL.EscapedPath()))
		assets = append(assets, githubReleaseAsset{
			Name:               name,
			BrowserDownloadURL: downloadURL,
			Size:               parseGitHubAssetSize(fragment, href),
			Digest:             parseGitHubAssetDigest(fragment, name),
		})
	}
	if len(assets) == 0 {
		return githubReleaseResponse{}, fmt.Errorf("no .yqpack assets found")
	}
	return githubReleaseResponse{
		TagName: parsed.tag,
		Name:    parsed.tag,
		HTMLURL: fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", url.PathEscape(parsed.owner), url.PathEscape(parsed.repo), strings.ReplaceAll(url.PathEscape(parsed.tag), "%2F", "/")),
		Assets:  assets,
	}, nil
}

type githubReleaseURLParts struct {
	owner string
	repo  string
	tag   string
}

func parseGitHubReleaseURL(releaseURL string) (githubReleaseURLParts, error) {
	parsed, err := url.Parse(strings.TrimSpace(releaseURL))
	if err != nil {
		return githubReleaseURLParts{}, fmt.Errorf("parse release url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return githubReleaseURLParts{}, fmt.Errorf("release url must use http or https")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" {
		return githubReleaseURLParts{}, fmt.Errorf("only github.com release urls are supported")
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) < 5 || parts[2] != "releases" || parts[3] != "tag" {
		return githubReleaseURLParts{}, fmt.Errorf("expected github release tag url")
	}
	owner, err := url.PathUnescape(parts[0])
	if err != nil {
		return githubReleaseURLParts{}, fmt.Errorf("parse github owner: %w", err)
	}
	repo, err := url.PathUnescape(parts[1])
	if err != nil {
		return githubReleaseURLParts{}, fmt.Errorf("parse github repo: %w", err)
	}
	tagEscaped := strings.Join(parts[4:], "/")
	tag, err := url.PathUnescape(tagEscaped)
	if err != nil {
		return githubReleaseURLParts{}, fmt.Errorf("parse github release tag: %w", err)
	}
	return githubReleaseURLParts{owner: owner, repo: repo, tag: tag}, nil
}

func htmlUnescape(value string) string {
	replacer := strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">")
	return replacer.Replace(value)
}

func pathfileBase(escapedPath string) string {
	parts := strings.Split(strings.TrimRight(escapedPath, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func parseGitHubAssetDigest(fragment string, assetName string) string {
	if strings.TrimSpace(assetName) == "" {
		return ""
	}
	idx := strings.Index(fragment, assetName)
	if idx < 0 {
		return ""
	}
	end := idx + 4096
	if end > len(fragment) {
		end = len(fragment)
	}
	window := fragment[idx:end]
	digestPattern := regexp.MustCompile(`sha256:[a-fA-F0-9]{64}`)
	return digestPattern.FindString(window)
}

func parseGitHubAssetSize(fragment string, href string) int64 {
	idx := strings.Index(fragment, href)
	if idx < 0 {
		idx = 0
	}
	end := idx + 4096
	if end > len(fragment) {
		end = len(fragment)
	}
	window := fragment[idx:end]
	sizePattern := regexp.MustCompile(`>([0-9]+(?:\.[0-9]+)?)\s*(KB|MB|GB|bytes)<`)
	match := sizePattern.FindStringSubmatch(window)
	if len(match) < 3 {
		return 0
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	switch strings.ToUpper(match[2]) {
	case "GB":
		value *= 1024 * 1024 * 1024
	case "MB":
		value *= 1024 * 1024
	case "KB":
		value *= 1024
	}
	return int64(value)
}

func githubReleaseAPIURL(releaseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(releaseURL))
	if err != nil {
		return "", fmt.Errorf("parse release url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("release url must use http or https")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "api.github.com" {
		return parsed.String(), nil
	}
	if host != "github.com" {
		return "", fmt.Errorf("only github.com release urls are supported")
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) < 5 || parts[2] != "releases" || parts[3] != "tag" {
		return "", fmt.Errorf("expected github release tag url")
	}
	owner, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", fmt.Errorf("parse github owner: %w", err)
	}
	repo, err := url.PathUnescape(parts[1])
	if err != nil {
		return "", fmt.Errorf("parse github repo: %w", err)
	}
	tagEscaped := strings.Join(parts[4:], "/")
	tag, err := url.PathUnescape(tagEscaped)
	if err != nil {
		return "", fmt.Errorf("parse github release tag: %w", err)
	}
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(tag)), nil
}

func fetchYqpackManifest(r *http.Request, packageURL string) (packruntime.Manifest, string, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, packageURL, nil)
	if err != nil {
		return packruntime.Manifest{}, "", fmt.Errorf("create yqpack request: %w", err)
	}
	req.Header.Set("User-Agent", "Yunque-PackRuntime")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return packruntime.Manifest{}, "", fmt.Errorf("download yqpack: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return packruntime.Manifest{}, "", fmt.Errorf("download yqpack: http %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return packruntime.Manifest{}, "", fmt.Errorf("read yqpack: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return packruntime.Manifest{}, "", fmt.Errorf("downloaded yqpack is empty")
	}
	manifest, digest, err := packruntime.InspectYqpackManifestBytes(data)
	if err != nil {
		return packruntime.Manifest{}, digest, err
	}
	return manifest, digest, nil
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
		ManifestURL:  manifest.Distribution.ManifestURL,
		PackageURL:   manifest.Distribution.PackageURL,
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

func (g *Gateway) handlePackCapabilityCogniProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	writeJSON(w, g.packCapabilityCogniProfileReport())
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

func (g *Gateway) packCapabilityCogniProfileReport() packruntime.CapabilityCogniProfileReport {
	index := g.packCapabilityIndexReport()
	report := packruntime.CapabilityCogniProfileReport{
		GeneratedAt: index.GeneratedAt,
		Entries:     []packruntime.CapabilityCogniProfileEntry{},
	}
	for _, entry := range index.Entries {
		profile := packCapabilityCogniProfileEntry(entry)
		report.Entries = append(report.Entries, profile)
	}
	report.Count = len(report.Entries)
	return report
}

func packCapabilityCogniProfileEntry(entry packruntime.CapabilityIndexEntry) packruntime.CapabilityCogniProfileEntry {
	risk := packCapabilityCogniRisk(entry.Permissions)
	action := "enable"
	if entry.Enabled {
		action = "use"
	}
	constraints := packCapabilityCogniConstraints(entry, risk)
	return packruntime.CapabilityCogniProfileEntry{
		Capability:    entry.Capability,
		SourceType:    "pack",
		SourceID:      entry.PackID,
		SourceName:    entry.PackName,
		Enabled:       entry.Enabled,
		Action:        action,
		Risk:          risk,
		TokenHint:     packCapabilityTokenHint(entry),
		UseWhen:       packCapabilityUseWhen(entry),
		Constraints:   constraints,
		Permissions:   append([]string(nil), entry.Permissions...),
		FrontendPaths: append([]string(nil), entry.FrontendPaths...),
		InvokeHint:    packCapabilityInvokeHint(entry),
	}
}

func packCapabilityCogniRisk(permissions []string) string {
	joined := strings.ToLower(strings.Join(permissions, " "))
	switch {
	case strings.Contains(joined, "computer") || strings.Contains(joined, "desktop") || strings.Contains(joined, "browser:write") || strings.Contains(joined, "wasm:execute"):
		return "high"
	case strings.Contains(joined, "write") || strings.Contains(joined, "delete") || strings.Contains(joined, "network") || strings.Contains(joined, "download") || strings.Contains(joined, "admin"):
		return "medium"
	default:
		return "low"
	}
}

func packCapabilityTokenHint(entry packruntime.CapabilityIndexEntry) string {
	parts := []string{entry.Capability, "via", entry.PackID}
	if entry.Enabled {
		parts = append(parts, "enabled")
	} else {
		parts = append(parts, "disabled")
	}
	if len(entry.Permissions) > 0 {
		parts = append(parts, "perms="+strings.Join(entry.Permissions, ","))
	}
	return strings.Join(parts, " ")
}

func packCapabilityUseWhen(entry packruntime.CapabilityIndexEntry) []string {
	capability := strings.ToLower(entry.Capability)
	switch {
	case strings.Contains(capability, "memory") || strings.Contains(capability, "recall") || strings.Contains(capability, "knowledge"):
		return []string{"需要召回用户记忆、知识库或长期上下文时"}
	case strings.Contains(capability, "browser"):
		return []string{"需要读取、规划或检查浏览器页面时"}
	case strings.Contains(capability, "computer"):
		return []string{"需要生成电脑使用计划时；当前不代表可直接执行本机控制"}
	case strings.Contains(capability, "workflow") || strings.Contains(capability, "task"):
		return []string{"需要把用户目标转成任务、流程或可跟踪执行时"}
	case strings.Contains(capability, "wasm"):
		return []string{"需要加载隔离 WASM 能力或检查第三方执行器时"}
	default:
		return []string{"用户目标与该能力名称或能力包说明匹配时"}
	}
}

func packCapabilityCogniConstraints(entry packruntime.CapabilityIndexEntry, risk string) []string {
	constraints := []string{"按需展开完整能力说明，不要把完整 manifest 放入模型上下文"}
	if !entry.Enabled {
		constraints = append(constraints, "能力包未启用，使用前先引导启用")
	}
	if risk == "high" {
		constraints = append(constraints, "高风险能力需要用户授权，不能静默扩大权限")
	}
	if len(entry.Routes) > 0 {
		constraints = append(constraints, "只能调用能力包声明并通过审计的 route")
	}
	return constraints
}

func packCapabilityInvokeHint(entry packruntime.CapabilityIndexEntry) string {
	if entry.SDKTypeScript != "" {
		return "sdk:" + entry.SDKTypeScript
	}
	if len(entry.Routes) > 0 {
		return "route:" + entry.Routes[0]
	}
	if len(entry.FrontendPaths) > 0 {
		return "ui:" + entry.FrontendPaths[0]
	}
	return "pack:" + entry.PackID
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

func (g *Gateway) handlePackStudioPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req packruntime.PackStudioPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	manifest, opts, ok := g.resolvePackStudioTarget(req)
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, map[string]any{"error": "pack not found"})
		return
	}
	if err := manifest.Validate(); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "invalid pack manifest", "detail": err.Error()})
		return
	}
	opts.Goal = req.Goal
	writeJSON(w, packruntime.BuildPackStudioPlan(manifest, opts))
}

func (g *Gateway) handlePackStudioInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req packruntime.PackStudioInspectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	report, err := inspectPackStudioYqpack(r, req)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, report)
}

func (g *Gateway) handlePackStudioWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if g.packRegistry == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "pack registry not configured"})
		return
	}
	var req packruntime.PackStudioWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	report, err := g.preparePackStudioWorkspace(r, req)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, report)
}

func (g *Gateway) handlePackStudioPatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var req packruntime.PackStudioPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	report, err := packruntime.PatchStudioWorkspaceFile(req)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, report)
}

func inspectPackStudioYqpack(r *http.Request, req packruntime.PackStudioInspectRequest) (packruntime.YqpackInspectReport, error) {
	if strings.TrimSpace(req.PackagePath) != "" {
		return packruntime.InspectYqpackFile(req.PackagePath, req.SHA256, req.Goal)
	}
	packageURL := strings.TrimSpace(req.PackageURL)
	if packageURL == "" {
		return packruntime.YqpackInspectReport{}, fmt.Errorf("package_path or package_url is required")
	}
	data, err := downloadPackStudioYqpack(r, packageURL)
	if err != nil {
		return packruntime.YqpackInspectReport{}, err
	}
	return packruntime.InspectYqpackBytes(data, packageURL, req.SHA256, req.Goal)
}

func (g *Gateway) preparePackStudioWorkspace(r *http.Request, req packruntime.PackStudioWorkspaceRequest) (packruntime.PackStudioWorkspaceReport, error) {
	if strings.TrimSpace(req.PackagePath) != "" {
		return g.packRegistry.PrepareStudioWorkspaceFromYqpack(req.PackagePath, req.SHA256, req.Goal)
	}
	packageURL := strings.TrimSpace(req.PackageURL)
	if packageURL == "" {
		return packruntime.PackStudioWorkspaceReport{}, fmt.Errorf("package_path or package_url is required")
	}
	data, err := downloadPackStudioYqpack(r, packageURL)
	if err != nil {
		return packruntime.PackStudioWorkspaceReport{}, err
	}
	return g.packRegistry.PrepareStudioWorkspaceFromYqpackBytes(data, packageURL, req.SHA256, req.Goal)
}

func downloadPackStudioYqpack(r *http.Request, packageURL string) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, packageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create yqpack request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Yunque-PackRuntime")
	res, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("download yqpack: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("download yqpack: http %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("read yqpack: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("downloaded yqpack is empty")
	}
	return data, nil
}

func (g *Gateway) resolvePackStudioTarget(req packruntime.PackStudioPlanRequest) (packruntime.Manifest, packruntime.PackStudioPlanOptions, bool) {
	if req.Manifest != nil {
		manifest := *req.Manifest
		return manifest, packruntime.PackStudioPlanOptions{
			Source:    "request-manifest",
			Installed: false,
			Enabled:   false,
			Status:    strings.TrimSpace(manifest.Status),
		}, true
	}
	packID := strings.TrimSpace(req.PackID)
	if packID == "" {
		return packruntime.Manifest{}, packruntime.PackStudioPlanOptions{}, false
	}
	if g.packRegistry != nil {
		if pack, ok := g.packRegistry.Get(packID); ok {
			return pack.Manifest, packruntime.PackStudioPlanOptions{
				Source:    pack.Source,
				Installed: true,
				Enabled:   pack.Status == packruntime.PackStatusEnabled,
				Status:    string(pack.Status),
			}, true
		}
	}
	catalog := g.packCatalogReport("", "")
	for _, entry := range catalog.Entries {
		if entry.Manifest.ID != packID {
			continue
		}
		return entry.Manifest, packruntime.PackStudioPlanOptions{
			Source:    entry.Source,
			Installed: entry.Installed,
			Enabled:   entry.Enabled,
			Status:    string(entry.Status),
		}, true
	}
	return packruntime.Manifest{}, packruntime.PackStudioPlanOptions{}, false
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.ManifestPath == "" && req.ManifestURL == "" && req.PackageURL == "") {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "manifest_path, manifest_url or package_url is required"})
		return
	}
	var pack packruntime.InstalledPack
	if req.PackageURL != "" && req.ManifestPath == "" && req.ManifestURL == "" {
		pack, err := g.installPackFromPackageURL(r, req)
		if err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if g.planner != nil {
			g.planner.InvalidatePromptCache()
		}
		writeJSON(w, map[string]any{"pack": pack, "status": pack.Status})
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
	// A wasm-backed pack must be extracted to disk (its .wasm module is needed
	// at request time), so a downloaded .yqpack is installed via
	// InstallFromYqpack rather than registering the manifest alone.
	if artifacts != nil && strings.HasSuffix(strings.ToLower(artifacts.PackagePath), ".yqpack") {
		pack, err = g.packRegistry.InstallFromYqpack(artifacts.PackagePath, packruntime.InstallOptions{
			ExpectedSHA256: manifest.Distribution.SHA256,
			TrustRoot:      g.packTrustRoot,
			Source:         source,
		})
	} else {
		pack, err = g.packRegistry.InstallWithArtifacts(manifest, source, artifacts)
	}
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if g.planner != nil {
		g.planner.InvalidatePromptCache()
	}
	writeJSON(w, map[string]any{"pack": pack, "status": pack.Status})
}

func (g *Gateway) installPackFromPackageURL(r *http.Request, req packInstallRequest) (packruntime.InstalledPack, error) {
	manifest, artifactSHA, err := fetchYqpackManifest(r, req.PackageURL)
	if err != nil {
		return packruntime.InstalledPack{}, err
	}
	expected := strings.TrimSpace(req.SHA256)
	if expected == "" {
		expected = artifactSHA
	}
	manifest.Distribution.PackageURL = req.PackageURL
	manifest.Distribution.SHA256 = expected
	artifacts, err := g.packRegistry.CacheDistribution(r.Context(), manifest)
	if err != nil {
		return packruntime.InstalledPack{}, err
	}
	source := req.Source
	if source == "" {
		source = req.PackageURL
	}
	return g.packRegistry.InstallFromYqpack(artifacts.PackagePath, packruntime.InstallOptions{
		ExpectedSHA256: expected,
		TrustRoot:      g.packTrustRoot,
		Source:         source,
	})
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
	// Pack state changed → invalidate planner system-prompt cache so the next
	// turn rebuilds it with the new pack list (menus/capabilities/identity).
	if g.planner != nil {
		g.planner.InvalidatePromptCache()
	}
	writeJSON(w, map[string]any{"pack": pack, "status": pack.Status})
}
