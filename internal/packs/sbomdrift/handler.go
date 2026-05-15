package sbomdrift

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.sbom-drift"

type Config struct {
	RepoRoot string
	DataDir  string
	Now      func() time.Time
}

type Handler struct {
	repoRoot string
	dataDir  string
	now      func() time.Time
}

type Component struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Path      string `json:"path,omitempty"`
	Direct    bool   `json:"direct"`
}

type Snapshot struct {
	ID             string         `json:"id"`
	Source         string         `json:"source"`
	CreatedAt      time.Time      `json:"created_at"`
	ComponentCount int            `json:"component_count"`
	Ecosystems     map[string]int `json:"ecosystems"`
	Components     []Component    `json:"components"`
}

type SnapshotSummary struct {
	ID             string         `json:"id"`
	Source         string         `json:"source"`
	CreatedAt      time.Time      `json:"created_at"`
	ComponentCount int            `json:"component_count"`
	Ecosystems     map[string]int `json:"ecosystems"`
}

type DiffRequest struct {
	BaseID        string `json:"base_id"`
	TargetID      string `json:"target_id,omitempty"`
	TargetCurrent bool   `json:"target_current,omitempty"`
}

type ComponentChange struct {
	Ecosystem  string `json:"ecosystem"`
	Name       string `json:"name"`
	Path       string `json:"path,omitempty"`
	OldVersion string `json:"old_version,omitempty"`
	NewVersion string `json:"new_version,omitempty"`
	Risk       string `json:"risk"`
}

type DiffResult struct {
	Base      SnapshotSummary   `json:"base"`
	Target    SnapshotSummary   `json:"target"`
	Added     []ComponentChange `json:"added"`
	Removed   []ComponentChange `json:"removed"`
	Changed   []ComponentChange `json:"changed"`
	RiskLevel string            `json:"risk_level"`
	Notes     []string          `json:"notes,omitempty"`
}

var safeIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)

func New(cfg Config) *Handler {
	repoRoot := strings.TrimSpace(cfg.RepoRoot)
	if repoRoot == "" {
		repoRoot = "."
	}
	dataDir := strings.TrimSpace(cfg.DataDir)
	if dataDir == "" {
		dataDir = filepath.Join(".", "data", "sbom-drift")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{repoRoot: repoRoot, dataDir: dataDir, now: now}
}

func DefaultHandler() *Handler { return New(Config{}) }

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/sbom-drift/status", Handler: h.Status},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/sbom-drift/snapshots", Handler: h.Snapshots},
		{Method: http.MethodGet, Path: "/v1/sbom-drift/snapshots/", Handler: h.SnapshotDetail},
		{Method: http.MethodPost, Path: "/v1/sbom-drift/diff", Handler: h.Diff},
		{Method: http.MethodGet, Path: "/v1/sbom-drift/evidence/", Handler: h.Evidence},
	}
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshots, err := h.listSnapshots()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":             PackID,
		"stage":               "pack-shell-before-ci",
		"scanner_ready":       true,
		"vulnerability_ready": false,
		"snapshot_count":      len(snapshots),
		"repo_root":           h.repoRoot,
		"store_dir":           h.dataDir,
		"capabilities": []string{
			"sbom.snapshot.go_mod",
			"sbom.snapshot.npm_package_json",
			"sbom.drift.diff",
			"sbom.evidence.export",
		},
		"notes": []string{"CycloneDX generation and govulncheck CI gates are planned follow-up wiring."},
	})
}

func (h *Handler) Snapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		snapshots, err := h.listSnapshots()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"snapshots": snapshots, "count": len(snapshots)})
	case http.MethodPost:
		var req struct {
			ID     string `json:"id"`
			Source string `json:"source"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		snapshot, err := h.createSnapshot(req.ID, req.Source)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.saveSnapshot(snapshot); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"snapshot": snapshot, "status": "created"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) SnapshotDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/sbom-drift/snapshots/")
	snapshot, err := h.loadSnapshot(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshot": snapshot})
}

func (h *Handler) Diff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req DiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.BaseID) == "" {
		writeError(w, http.StatusBadRequest, "base_id is required")
		return
	}
	base, err := h.loadSnapshot(req.BaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("base snapshot not found: %s", req.BaseID))
		return
	}
	var target Snapshot
	if strings.TrimSpace(req.TargetID) != "" {
		target, err = h.loadSnapshot(req.TargetID)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("target snapshot not found: %s", req.TargetID))
			return
		}
	} else {
		target, err = h.createSnapshot("current", "working-tree")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"diff": diffSnapshots(base, target)})
}

func (h *Handler) Evidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/sbom-drift/evidence/")
	snapshot, err := h.loadSnapshot(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":     PackID,
		"exported_at": h.now().UTC(),
		"format":      "json-sbom-drift-evidence",
		"files":       []string{"snapshot.json", "meta.json"},
		"snapshot":    snapshot,
	})
}

func (h *Handler) createSnapshot(id string, source string) (Snapshot, error) {
	components, err := h.collectComponents()
	if err != nil {
		return Snapshot{}, err
	}
	if strings.TrimSpace(id) == "" || id == "current" {
		id = "snap-" + h.now().UTC().Format("20060102150405")
	}
	id = strings.ToLower(strings.TrimSpace(id))
	if !safeIDRe.MatchString(id) {
		return Snapshot{}, fmt.Errorf("snapshot id must match ^[a-z0-9][a-z0-9_-]{0,79}$")
	}
	if strings.TrimSpace(source) == "" {
		source = "working-tree"
	}
	sortComponents(components)
	ecosystems := map[string]int{}
	for _, component := range components {
		ecosystems[component.Ecosystem]++
	}
	return Snapshot{ID: id, Source: source, CreatedAt: h.now().UTC(), ComponentCount: len(components), Ecosystems: ecosystems, Components: components}, nil
}

func (h *Handler) collectComponents() ([]Component, error) {
	var out []Component
	goMod := filepath.Join(h.repoRoot, "go.mod")
	if components, err := readGoModComponents(goMod, h.repoRoot); err == nil {
		out = append(out, components...)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	packageJSONs, err := findPackageJSONs(h.repoRoot)
	if err != nil {
		return nil, err
	}
	for _, path := range packageJSONs {
		components, err := readPackageJSONComponents(path, h.repoRoot)
		if err != nil {
			return nil, err
		}
		out = append(out, components...)
	}
	return out, nil
}

func readGoModComponents(path string, repoRoot string) ([]Component, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var out []Component
	inRequire := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "require"))
		} else if !inRequire {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rel, _ := filepath.Rel(repoRoot, path)
		out = append(out, Component{Ecosystem: "gomod", Name: fields[0], Version: fields[1], Scope: goScope(line), Path: filepath.ToSlash(rel), Direct: !strings.Contains(line, "// indirect")})
	}
	return out, scanner.Err()
}

func goScope(line string) string {
	if strings.Contains(line, "// indirect") {
		return "indirect"
	}
	return "direct"
}

func findPackageJSONs(repoRoot string) ([]string, error) {
	var out []string
	skip := map[string]bool{".git": true, "node_modules": true, ".next": true, "dist": true, "build": true, ".vitepress": true, "coverage": true, ".tmp": true}
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "package.json" {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func readPackageJSONComponents(path string, repoRoot string) ([]Component, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Name                 string            `json:"name"`
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	rel, _ := filepath.Rel(repoRoot, path)
	rel = filepath.ToSlash(rel)
	var out []Component
	appendDeps := func(scope string, deps map[string]string) {
		for name, version := range deps {
			out = append(out, Component{Ecosystem: "npm", Name: name, Version: version, Scope: scope, Path: rel, Direct: scope != "peer"})
		}
	}
	appendDeps("dependencies", pkg.Dependencies)
	appendDeps("devDependencies", pkg.DevDependencies)
	appendDeps("optionalDependencies", pkg.OptionalDependencies)
	appendDeps("peer", pkg.PeerDependencies)
	return out, nil
}

func (h *Handler) snapshotRoot() string { return filepath.Join(h.dataDir, "snapshots") }

func (h *Handler) snapshotDir(id string) (string, error) {
	id = strings.Trim(strings.TrimSpace(id), "/")
	if !safeIDRe.MatchString(id) {
		return "", fmt.Errorf("invalid snapshot id")
	}
	return filepath.Join(h.snapshotRoot(), id), nil
}

func (h *Handler) saveSnapshot(snapshot Snapshot) error {
	dir, err := h.snapshotDir(snapshot.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	meta, err := json.MarshalIndent(toSummary(snapshot), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), append(meta, '\n'), 0o644)
}

func (h *Handler) loadSnapshot(id string) (Snapshot, error) {
	dir, err := h.snapshotDir(id)
	if err != nil {
		return Snapshot{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot not found")
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("invalid snapshot file")
	}
	return snapshot, nil
}

func (h *Handler) listSnapshots() ([]SnapshotSummary, error) {
	entries, err := os.ReadDir(h.snapshotRoot())
	if os.IsNotExist(err) {
		return []SnapshotSummary{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]SnapshotSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !safeIDRe.MatchString(entry.Name()) {
			continue
		}
		snapshot, err := h.loadSnapshot(entry.Name())
		if err == nil {
			out = append(out, toSummary(snapshot))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func diffSnapshots(base Snapshot, target Snapshot) DiffResult {
	baseMap := componentMap(base.Components)
	targetMap := componentMap(target.Components)
	var added, removed, changed []ComponentChange
	risk := "none"
	for key, targetComponent := range targetMap {
		baseComponent, ok := baseMap[key]
		if !ok {
			change := ComponentChange{Ecosystem: targetComponent.Ecosystem, Name: targetComponent.Name, Path: targetComponent.Path, NewVersion: targetComponent.Version, Risk: addedRisk(targetComponent)}
			added = append(added, change)
			risk = maxRisk(risk, change.Risk)
			continue
		}
		if baseComponent.Version != targetComponent.Version {
			change := ComponentChange{Ecosystem: targetComponent.Ecosystem, Name: targetComponent.Name, Path: targetComponent.Path, OldVersion: baseComponent.Version, NewVersion: targetComponent.Version, Risk: versionRisk(baseComponent.Version, targetComponent.Version)}
			changed = append(changed, change)
			risk = maxRisk(risk, change.Risk)
		}
	}
	for key, baseComponent := range baseMap {
		if _, ok := targetMap[key]; !ok {
			change := ComponentChange{Ecosystem: baseComponent.Ecosystem, Name: baseComponent.Name, Path: baseComponent.Path, OldVersion: baseComponent.Version, Risk: "low"}
			removed = append(removed, change)
			risk = maxRisk(risk, change.Risk)
		}
	}
	sortChanges(added)
	sortChanges(removed)
	sortChanges(changed)
	return DiffResult{Base: toSummary(base), Target: toSummary(target), Added: added, Removed: removed, Changed: changed, RiskLevel: risk, Notes: []string{"Known-CVE blocking is not connected yet; this pack shell reports dependency drift only."}}
}

func componentMap(components []Component) map[string]Component {
	out := make(map[string]Component, len(components))
	for _, component := range components {
		out[component.Ecosystem+":"+component.Name] = component
	}
	return out
}

func toSummary(snapshot Snapshot) SnapshotSummary {
	return SnapshotSummary{ID: snapshot.ID, Source: snapshot.Source, CreatedAt: snapshot.CreatedAt, ComponentCount: snapshot.ComponentCount, Ecosystems: snapshot.Ecosystems}
}

func sortComponents(components []Component) {
	sort.Slice(components, func(i, j int) bool {
		if components[i].Ecosystem != components[j].Ecosystem {
			return components[i].Ecosystem < components[j].Ecosystem
		}
		if components[i].Name != components[j].Name {
			return components[i].Name < components[j].Name
		}
		return components[i].Path < components[j].Path
	})
}

func sortChanges(changes []ComponentChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Ecosystem != changes[j].Ecosystem {
			return changes[i].Ecosystem < changes[j].Ecosystem
		}
		if changes[i].Name != changes[j].Name {
			return changes[i].Name < changes[j].Name
		}
		return changes[i].Path < changes[j].Path
	})
}

func addedRisk(component Component) string {
	if component.Direct {
		return "high"
	}
	return "medium"
}

func versionRisk(oldVersion string, newVersion string) string {
	oldMajor := semverMajor(oldVersion)
	newMajor := semverMajor(newVersion)
	if oldMajor != "" && newMajor != "" && oldMajor != newMajor {
		return "high"
	}
	if oldVersion != newVersion {
		return "medium"
	}
	return "none"
}

func semverMajor(version string) string {
	version = strings.TrimLeft(version, "^~>=<v ")
	parts := strings.Split(version, ".")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0]
}

func maxRisk(a string, b string) string {
	order := map[string]int{"none": 0, "low": 1, "medium": 2, "high": 3, "critical": 4}
	if order[b] > order[a] {
		return b
	}
	return a
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
