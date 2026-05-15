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

type CycloneDXDocument struct {
	BOMFormat    string                `json:"bomFormat"`
	SpecVersion  string                `json:"specVersion"`
	Version      int                   `json:"version"`
	Metadata     CycloneDXMetadata     `json:"metadata"`
	Components   []CycloneDXComponent  `json:"components"`
	Dependencies []CycloneDXDependency `json:"dependencies,omitempty"`
}

type CycloneDXMetadata struct {
	Timestamp time.Time          `json:"timestamp"`
	Component CycloneDXComponent `json:"component"`
	Tools     []CycloneDXTool    `json:"tools,omitempty"`
}

type CycloneDXTool struct {
	Vendor  string `json:"vendor,omitempty"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type CycloneDXComponent struct {
	Type       string              `json:"type"`
	BOMRef     string              `json:"bom-ref,omitempty"`
	Name       string              `json:"name"`
	Version    string              `json:"version,omitempty"`
	Scope      string              `json:"scope,omitempty"`
	PURL       string              `json:"purl,omitempty"`
	Properties []CycloneDXProperty `json:"properties,omitempty"`
}

type CycloneDXProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CycloneDXDependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

type CIGatePlanRequest struct {
	BaseID        string `json:"base_id"`
	TargetID      string `json:"target_id,omitempty"`
	TargetCurrent bool   `json:"target_current,omitempty"`
	FailOnRisk    string `json:"fail_on_risk,omitempty"`
	RequestedBy   string `json:"requested_by,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type CIGatePlanReport struct {
	PackID               string          `json:"pack_id"`
	GeneratedAt          time.Time       `json:"generated_at"`
	Status               string          `json:"status"`
	Blocked              bool            `json:"blocked"`
	FailOnRisk           string          `json:"fail_on_risk"`
	CycloneDXReady       bool            `json:"cyclonedx_ready"`
	CIGatePlanReady      bool            `json:"ci_gate_plan_ready"`
	CIGateReady          bool            `json:"ci_gate_ready"`
	GovulncheckPlanReady bool            `json:"govulncheck_plan_ready"`
	GovulncheckReady     bool            `json:"govulncheck_ready"`
	RequestedBy          string          `json:"requested_by,omitempty"`
	Reason               string          `json:"reason,omitempty"`
	Diff                 DiffResult      `json:"diff"`
	GovulncheckPlan      GovulncheckPlan `json:"govulncheck_plan"`
	Artifacts            []string        `json:"artifacts"`
	Commands             []string        `json:"commands"`
	Actions              []string        `json:"actions"`
	Notes                []string        `json:"notes,omitempty"`
}

type GovulncheckPlan struct {
	PlanReady            bool                     `json:"plan_ready"`
	Ready                bool                     `json:"ready"`
	Status               string                   `json:"status"`
	Command              string                   `json:"command"`
	TargetPackage        string                   `json:"target_package"`
	ReportArtifact       string                   `json:"report_artifact"`
	Executes             bool                     `json:"executes"`
	WritesFiles          bool                     `json:"writes_files"`
	VulnerabilityDBFetch bool                     `json:"vulnerability_db_fetch"`
	PackageCount         int                      `json:"package_count"`
	ModuleCount          int                      `json:"module_count"`
	Packages             []GovulncheckPackagePlan `json:"packages"`
	Labels               []string                 `json:"labels"`
	Notes                []string                 `json:"notes,omitempty"`
}

type GovulncheckPackagePlan struct {
	Ecosystem string   `json:"ecosystem"`
	Module    string   `json:"module"`
	Version   string   `json:"version,omitempty"`
	Scope     string   `json:"scope,omitempty"`
	Path      string   `json:"path,omitempty"`
	Direct    bool     `json:"direct"`
	Labels    []string `json:"labels,omitempty"`
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
		{Method: http.MethodGet, Path: "/v1/sbom-drift/cyclonedx/", Handler: h.CycloneDX},
		{Method: http.MethodPost, Path: "/v1/sbom-drift/ci-gate/plan", Handler: h.CIGatePlan},
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
		"pack_id":                PackID,
		"stage":                  "pack-shell-before-ci",
		"scanner_ready":          true,
		"cyclonedx_ready":        true,
		"ci_gate_plan_ready":     true,
		"ci_gate_ready":          false,
		"vulnerability_ready":    false,
		"govulncheck_plan_ready": true,
		"govulncheck_ready":      false,
		"snapshot_count":         len(snapshots),
		"repo_root":              h.repoRoot,
		"store_dir":              h.dataDir,
		"capabilities": []string{
			"sbom.snapshot.go_mod",
			"sbom.snapshot.npm_package_json",
			"sbom.drift.diff",
			"sbom.cyclonedx.export",
			"sbom.ci_gate.plan",
			"sbom.govulncheck.plan",
			"sbom.evidence.export",
		},
		"notes": []string{"CycloneDX JSON export, CI gate plan, and govulncheck command preview are available as non-destructive pack contracts; govulncheck execution and CI write-back remain follow-up wiring."},
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

func (h *Handler) CycloneDX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/sbom-drift/cyclonedx/")
	snapshot, err := h.loadSnapshotOrCurrent(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bom": h.buildCycloneDX(snapshot), "snapshot": toSummary(snapshot)})
}

func (h *Handler) CIGatePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req CIGatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.BaseID) == "" {
		writeError(w, http.StatusBadRequest, "base_id is required")
		return
	}
	base, err := h.loadSnapshot(req.BaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("base snapshot not found: %s", req.BaseID))
		return
	}
	target, err := h.resolveTargetSnapshot(req.TargetID, req.TargetCurrent)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	plan := h.buildCIGatePlan(diffSnapshots(base, target), target, req.FailOnRisk, req.RequestedBy, req.Reason)
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
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
	ciGatePlan := h.buildCIGatePlan(diffSnapshots(snapshot, snapshot), snapshot, "high", "evidence-export", "snapshot evidence schema snapshot")
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":          PackID,
		"exported_at":      h.now().UTC(),
		"format":           "json-sbom-drift-evidence",
		"files":            []string{"snapshot.json", "meta.json", "sbom.cdx.json", "ci-gate-plan.json", "govulncheck-plan.json"},
		"snapshot":         snapshot,
		"cyclonedx":        h.buildCycloneDX(snapshot),
		"ci_gate_plan":     ciGatePlan,
		"govulncheck_plan": ciGatePlan.GovulncheckPlan,
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

func (h *Handler) loadSnapshotOrCurrent(id string) (Snapshot, error) {
	id = strings.Trim(strings.TrimSpace(id), "/")
	if id == "" || id == "current" {
		return h.createSnapshot("current", "working-tree")
	}
	return h.loadSnapshot(id)
}

func (h *Handler) resolveTargetSnapshot(targetID string, targetCurrent bool) (Snapshot, error) {
	if strings.TrimSpace(targetID) != "" {
		target, err := h.loadSnapshot(targetID)
		if err != nil {
			return Snapshot{}, fmt.Errorf("target snapshot not found: %s", targetID)
		}
		return target, nil
	}
	if targetCurrent || strings.TrimSpace(targetID) == "" {
		return h.createSnapshot("current", "working-tree")
	}
	return Snapshot{}, fmt.Errorf("target snapshot not found")
}

func (h *Handler) buildCycloneDX(snapshot Snapshot) CycloneDXDocument {
	components := make([]CycloneDXComponent, 0, len(snapshot.Components))
	dependsOn := make([]string, 0, len(snapshot.Components))
	for _, component := range snapshot.Components {
		ref := cyclonedxRef(component)
		dependsOn = append(dependsOn, ref)
		properties := []CycloneDXProperty{
			{Name: "yunque:ecosystem", Value: component.Ecosystem},
			{Name: "yunque:direct", Value: fmt.Sprintf("%t", component.Direct)},
		}
		if component.Path != "" {
			properties = append(properties, CycloneDXProperty{Name: "yunque:path", Value: component.Path})
		}
		components = append(components, CycloneDXComponent{
			Type:       "library",
			BOMRef:     ref,
			Name:       component.Name,
			Version:    component.Version,
			Scope:      cyclonedxScope(component.Scope),
			PURL:       packageURL(component),
			Properties: properties,
		})
	}
	return CycloneDXDocument{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Metadata: CycloneDXMetadata{
			Timestamp: snapshot.CreatedAt.UTC(),
			Component: CycloneDXComponent{
				Type:    "application",
				BOMRef:  "pkg:generic/yunque-agent@" + snapshot.ID,
				Name:    "yunque-agent",
				Version: snapshot.ID,
			},
			Tools: []CycloneDXTool{{Vendor: "Yunque", Name: "sbom-drift-pack", Version: "0.1.0"}},
		},
		Components:   components,
		Dependencies: []CycloneDXDependency{{Ref: "pkg:generic/yunque-agent@" + snapshot.ID, DependsOn: dependsOn}},
	}
}

func (h *Handler) buildCIGatePlan(diff DiffResult, target Snapshot, failOnRisk, requestedBy, reason string) CIGatePlanReport {
	threshold := normalizeRisk(failOnRisk)
	if threshold == "" {
		threshold = "high"
	}
	blocked := riskRank(diff.RiskLevel) >= riskRank(threshold) && diff.RiskLevel != "none"
	status := "ci_gate_pass_plan"
	if blocked {
		status = "ci_gate_block_plan"
	}
	actions := []string{
		"would export CycloneDX JSON as dist/sbom.cdx.json during release packaging",
		"would compare the generated SBOM against the selected baseline before release",
		"would run govulncheck -json ./... and attach govulncheck-report.json to release evidence",
	}
	if blocked {
		actions = append(actions, fmt.Sprintf("would block release because risk %s reaches threshold %s", diff.RiskLevel, threshold))
	} else {
		actions = append(actions, fmt.Sprintf("would allow release because risk %s is below threshold %s", diff.RiskLevel, threshold))
	}
	govulncheckPlan := buildGovulncheckPlan(target)
	return CIGatePlanReport{
		PackID:               PackID,
		GeneratedAt:          h.now().UTC(),
		Status:               status,
		Blocked:              blocked,
		FailOnRisk:           threshold,
		CycloneDXReady:       true,
		CIGatePlanReady:      true,
		CIGateReady:          false,
		GovulncheckPlanReady: true,
		GovulncheckReady:     false,
		RequestedBy:          strings.TrimSpace(requestedBy),
		Reason:               strings.TrimSpace(reason),
		Diff:                 diff,
		GovulncheckPlan:      govulncheckPlan,
		Artifacts:            []string{"dist/sbom.cdx.json", "sbom-drift-report.json", "ci-gate-plan.json", "govulncheck-plan.json", govulncheckPlan.ReportArtifact},
		Commands:             []string{"make sbom", "govulncheck -json ./... > govulncheck-report.json", "node scripts/check-pack-runtime-all.mjs"},
		Actions:              actions,
		Notes: []string{
			"This route is non-destructive: it does not write CI workflow files, invoke govulncheck, fetch the vulnerability database, or block a release by itself.",
			"Use the plan shape as the contract for the later CI baseline gate and govulncheck runtime write-back slice.",
		},
	}
}

func buildGovulncheckPlan(snapshot Snapshot) GovulncheckPlan {
	packages := make([]GovulncheckPackagePlan, 0)
	moduleSet := map[string]struct{}{}
	for _, component := range snapshot.Components {
		if component.Ecosystem != "gomod" {
			continue
		}
		moduleSet[component.Name] = struct{}{}
		labels := []string{"gomod"}
		if component.Direct {
			labels = append(labels, "direct")
		} else {
			labels = append(labels, "indirect")
		}
		if component.Path != "" {
			labels = append(labels, "path:"+component.Path)
		}
		packages = append(packages, GovulncheckPackagePlan{
			Ecosystem: component.Ecosystem,
			Module:    component.Name,
			Version:   component.Version,
			Scope:     component.Scope,
			Path:      component.Path,
			Direct:    component.Direct,
			Labels:    labels,
		})
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Module != packages[j].Module {
			return packages[i].Module < packages[j].Module
		}
		if packages[i].Version != packages[j].Version {
			return packages[i].Version < packages[j].Version
		}
		return packages[i].Path < packages[j].Path
	})
	return GovulncheckPlan{
		PlanReady:            true,
		Ready:                false,
		Status:               "plan_only",
		Command:              "govulncheck -json ./...",
		TargetPackage:        "./...",
		ReportArtifact:       "govulncheck-report.json",
		Executes:             false,
		WritesFiles:          false,
		VulnerabilityDBFetch: false,
		PackageCount:         len(packages),
		ModuleCount:          len(moduleSet),
		Packages:             packages,
		Labels:               []string{"plan-only", "govulncheck", "no-exec", "no-file-write"},
		Notes: []string{
			"Preview only: the pack does not execute govulncheck or fetch vulnerability data in this route.",
			"Only Go module components are included; npm vulnerability scanning remains a separate future scanner slice.",
		},
	}
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

func cyclonedxRef(component Component) string {
	version := component.Version
	if version == "" {
		version = "unknown"
	}
	return fmt.Sprintf("pkg:%s/%s@%s", cyclonedxType(component.Ecosystem), component.Name, version)
}

func cyclonedxType(ecosystem string) string {
	switch ecosystem {
	case "gomod":
		return "golang"
	case "npm":
		return "npm"
	default:
		return "generic"
	}
}

func packageURL(component Component) string {
	version := component.Version
	if version == "" {
		return fmt.Sprintf("pkg:%s/%s", cyclonedxType(component.Ecosystem), component.Name)
	}
	return fmt.Sprintf("pkg:%s/%s@%s", cyclonedxType(component.Ecosystem), component.Name, version)
}

func cyclonedxScope(scope string) string {
	switch scope {
	case "devDependencies":
		return "optional"
	case "peer":
		return "optional"
	default:
		return "required"
	}
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
	if riskRank(b) > riskRank(a) {
		return b
	}
	return a
}

func normalizeRisk(risk string) string {
	risk = strings.ToLower(strings.TrimSpace(risk))
	if _, ok := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}[risk]; ok {
		return risk
	}
	return ""
}

func riskRank(risk string) int {
	order := map[string]int{"none": 0, "low": 1, "medium": 2, "high": 3, "critical": 4}
	return order[strings.ToLower(strings.TrimSpace(risk))]
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
