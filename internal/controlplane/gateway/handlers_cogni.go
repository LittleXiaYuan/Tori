package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/cogni"
)

// handleCognis serves both /v1/cognis (collection) and /v1/cognis/ (with
// optional sub-resource).
//
// Routes:
//
//	GET    /v1/cognis              → list every registered declaration
//	POST   /v1/cognis              → add an inline declaration (JSON body)
//	GET    /v1/cognis/{id}         → fetch one declaration
//	DELETE /v1/cognis/{id}         → remove one declaration
//	POST   /v1/cognis/{id}/enable  → enable
//	POST   /v1/cognis/{id}/disable → disable
//	POST   /v1/cognis/reload       → re-scan the cognis directory on disk
//	POST   /v1/cognis/import       → import a bundle (persists added/updated to disk)
//	GET    /v1/cognis/export       → export declarations as a bundle
//	GET    /v1/cognis/traces       → recent per-turn evaluation traces
//	GET    /v1/cognis/stats        → activation counts per cogni
//	GET    /v1/cognis/health       → health metrics for every cogni seen recently
//	GET    /v1/cognis/{id}/trace   → traces filtered to one cogni id
//	GET    /v1/cognis/{id}/health  → health rollup for one cogni
func (g *Gateway) handleCognis(w http.ResponseWriter, r *http.Request) {
	if g.cogniRegistry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/cognis")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "":
		g.cogniCollection(w, r)
	case path == "runtime/pack-state":
		if g.cogniKernelRuntimeState == nil {
			apperror.WriteCode(w, apperror.CodeInternal, "cogni runtime state reporter not configured")
			return
		}
		g.cogniKernelRuntimeState(w, r)
	case path == "reload":
		g.cogniReload(w, r)
	case path == "traces":
		g.cogniTracesAll(w, r)
	case path == "stats":
		g.cogniTraceStats(w, r)
	case path == "health":
		g.cogniHealthAll(w, r)
	case path == "alerts":
		g.cogniAlerts(w, r)
	case path == "alerts/scan":
		g.cogniAlertsScan(w, r)
	case path == "verify":
		g.cogniVerifyAll(w, r)
	case path == "generate":
		g.cogniGenerate(w, r)
	case path == "export":
		g.cogniExportBundle(w, r)
	case path == "import":
		g.cogniImportBundle(w, r)
	case path == "evolution":
		g.cogniEvolutionList(w, r)
	default:
		segs := strings.SplitN(path, "/", 3)
		id := segs[0]
		switch {
		case len(segs) == 1:
			g.cogniByID(w, r, id)
		case len(segs) == 2 && segs[1] == "enable":
			g.cogniSetEnabled(w, r, id, true)
		case len(segs) == 2 && segs[1] == "disable":
			g.cogniSetEnabled(w, r, id, false)
		case len(segs) == 2 && segs[1] == "trace":
			g.cogniTracesByID(w, r, id)
		case len(segs) == 2 && segs[1] == "health":
			g.cogniHealthByID(w, r, id)
		case len(segs) == 2 && segs[1] == "verify":
			g.cogniVerifyByID(w, r, id)
		case len(segs) == 2 && segs[1] == "workflows":
			g.cogniWorkflowsList(w, r, id)
		case len(segs) >= 2 && segs[1] == "workflow":
			g.cogniWorkflowRun(w, r, id, segs)
		case len(segs) == 2 && segs[1] == "experience":
			g.cogniExperience(w, r, id)
		case len(segs) == 3 && segs[1] == "experience" && segs[2] == "record":
			g.cogniExperienceRecord(w, r, id)
		case len(segs) == 3 && segs[1] == "experience" && strings.HasPrefix(segs[2], "patterns/"):
			g.cogniExperiencePatternRoute(w, r, id, segs[2])
		case len(segs) == 2 && segs[1] == "evolve":
			g.cogniEvolve(w, r, id)
		case len(segs) == 2 && segs[1] == "evolution":
			g.cogniEvolutionByID(w, r, id)
		case len(segs) == 2 && segs[1] == "expose":
			g.cogniFederationExpose(w, r, id, true)
		case len(segs) == 2 && segs[1] == "unexpose":
			g.cogniFederationExpose(w, r, id, false)
		default:
			apperror.WriteCode(w, apperror.CodeNotFound, "unknown cogni sub-resource")
		}
	}
}

// ServeCogniKernel is the temporary Gateway adapter for the Cogni Kernel pack's
// API interface. Pack Runtime owns the public /v1/cognis* route mounting and
// gates; Gateway only supplies existing business operations until those handlers
// are extracted behind a standalone Cogni service in later reversible steps.
func (g *Gateway) ServeCogniKernel(w http.ResponseWriter, r *http.Request) {
	g.handleCognis(w, r)
}

func (g *Gateway) cogniHealthAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniTraces == nil {
		json.NewEncoder(w).Encode(map[string]any{"health": []any{}, "count": 0})
		return
	}
	mon := cogni.NewMonitor(g.cogniTraces)
	out := mon.ComputeAll(traceLimit(r))
	json.NewEncoder(w).Encode(map[string]any{
		"health": out,
		"count":  len(out),
	})
}

func (g *Gateway) cogniAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniSentinel == nil {
		json.NewEncoder(w).Encode(map[string]any{"alerts": []any{}, "count": 0})
		return
	}
	alerts := g.cogniSentinel.Alerts()
	json.NewEncoder(w).Encode(map[string]any{"alerts": alerts, "count": len(alerts)})
}

func (g *Gateway) cogniAlertsScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniSentinel == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni sentinel not configured")
		return
	}
	alerts := g.cogniSentinel.Scan()
	json.NewEncoder(w).Encode(map[string]any{"alerts": alerts, "count": len(alerts)})
}

func (g *Gateway) cogniExportBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
		return
	}
	if g.cogniRegistry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}

	idsRaw := r.URL.Query().Get("ids")
	var ids []string
	if idsRaw != "" {
		for _, id := range strings.Split(idsRaw, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				ids = append(ids, id)
			}
		}
	}
	notes := r.URL.Query().Get("notes")

	bundle := g.cogniRegistry.ExportBundle(ids, notes)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"cogni-bundle.json\"")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(bundle)
}

// cogniImportBundle imports a bundle of Cogni declarations into the registry.
// Successfully imported cognis (added and updated) are automatically persisted
// to disk in the cogniDir as {id}.json files, ensuring they survive restarts.
// Skipped and failed cognis are not persisted.
//
// Query parameters:
//   - overwrite=true: replace existing declarations with the same ID
//   - overwrite=false (default): skip existing declarations
func (g *Gateway) cogniImportBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniRegistry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}
	var bundle cogni.Bundle
	if err := json.NewDecoder(r.Body).Decode(&bundle); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid bundle JSON: "+err.Error())
		return
	}
	overwrite := strings.EqualFold(r.URL.Query().Get("overwrite"), "true")
	summary, err := g.cogniRegistry.ImportBundle(&bundle, overwrite)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	// Persist successfully imported cognis to disk
	if g.cogniDir != "" {
		// Ensure the directory exists
		if err := os.MkdirAll(g.cogniDir, 0o755); err != nil {
			slog.Warn("cogni: failed to create directory", "dir", g.cogniDir, "err", err)
		} else {
			// Save added cognis
			for _, id := range summary.Added {
				if decl, ok := g.cogniRegistry.Get(id); ok {
					savePath := filepath.Join(g.cogniDir, id+".json")
					if err := cogni.SaveDeclaration(decl, savePath); err != nil {
						slog.Warn("cogni: failed to save imported declaration", "id", id, "path", savePath, "err", err)
					}
				}
			}
			// Save updated cognis
			for _, id := range summary.Updated {
				if decl, ok := g.cogniRegistry.Get(id); ok {
					savePath := filepath.Join(g.cogniDir, id+".json")
					if err := cogni.SaveDeclaration(decl, savePath); err != nil {
						slog.Warn("cogni: failed to save updated declaration", "id", id, "path", savePath, "err", err)
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (g *Gateway) cogniVerifyAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniRegistry == nil {
		json.NewEncoder(w).Encode(map[string]any{"results": map[string]any{}, "failures": []any{}})
		return
	}
	results := g.cogniRegistry.VerifyAll()
	json.NewEncoder(w).Encode(map[string]any{
		"results":  results,
		"failures": cogni.FailedChecks(results),
	})
}

func (g *Gateway) cogniVerifyByID(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniRegistry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni registry not configured")
		return
	}
	decl, ok := g.cogniRegistry.Get(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
		return
	}
	results := cogni.VerifyDeclaration(decl, nil)
	passed, failed := 0, 0
	for _, r := range results {
		if r.Passed {
			passed++
		} else if r.Reason != "no assertion configured (ignored)" {
			failed++
		}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"id":      id,
		"results": results,
		"passed":  passed,
		"failed":  failed,
	})
}

func (g *Gateway) cogniHealthByID(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniTraces == nil {
		json.NewEncoder(w).Encode(cogni.HealthMetrics{ID: id, Status: "idle"})
		return
	}
	json.NewEncoder(w).Encode(cogni.NewMonitor(g.cogniTraces).ComputeFor(id, traceLimit(r)))
}

func (g *Gateway) cogniTracesAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	limit := traceLimit(r)
	w.Header().Set("Content-Type", "application/json")
	if g.cogniTraces == nil {
		json.NewEncoder(w).Encode(map[string]any{"traces": []any{}, "count": 0})
		return
	}
	traces := g.cogniTraces.Recent(limit)
	json.NewEncoder(w).Encode(map[string]any{
		"traces": traces,
		"count":  len(traces),
	})
}

func (g *Gateway) cogniTracesByID(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniTraces == nil {
		json.NewEncoder(w).Encode(map[string]any{"traces": []any{}, "count": 0})
		return
	}
	limit := traceLimit(r)
	traces := g.cogniTraces.ByCogni(id, limit)
	json.NewEncoder(w).Encode(map[string]any{
		"id":     id,
		"traces": traces,
		"count":  len(traces),
	})
}

func (g *Gateway) cogniTraceStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniTraces == nil {
		json.NewEncoder(w).Encode(cogni.TraceStats{})
		return
	}
	json.NewEncoder(w).Encode(g.cogniTraces.Stats())
}

// traceLimit reads ?limit=N (defaults to 50, capped at 500).
func traceLimit(r *http.Request) int {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}
	return limit
}

func (g *Gateway) cogniCollection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		entries := g.cogniRegistry.List()
		health := map[string]cogni.HealthMetrics{}
		if g.cogniTraces != nil {
			mon := cogni.NewMonitor(g.cogniTraces)
			for _, hm := range mon.ComputeAll(0) {
				health[hm.ID] = hm
			}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"cognis":  entries,
			"health":  health,
			"count":   len(entries),
			"version": g.cogniRegistry.Version(),
			"dir":     g.cogniDir,
		})
	case http.MethodPost:
		var d cogni.Declaration
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body: "+err.Error())
			return
		}
		if err := g.cogniRegistry.Add(&d, "api"); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "id": d.ID})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
	}
}

func (g *Gateway) cogniByID(w http.ResponseWriter, r *http.Request, id string) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		decl, ok := g.cogniRegistry.Get(id)
		if !ok {
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":          decl.ID,
			"declaration": decl,
			"enabled":     g.cogniRegistry.IsEnabled(id),
		})
	case http.MethodDelete:
		if !g.cogniRegistry.Remove(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "removed", "id": id})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or DELETE")
	}
}

func (g *Gateway) cogniSetEnabled(w http.ResponseWriter, r *http.Request, id string, enabled bool) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if err := g.cogniRegistry.SetEnabled(id, enabled); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": state, "id": id})
}

func (g *Gateway) cogniReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	dir := g.cogniDir
	if dir == "" {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni directory not configured")
		return
	}
	summary, err := g.cogniRegistry.ReloadFromDir(dir)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	errs := make([]map[string]string, 0, len(summary.Errors))
	for _, e := range summary.Errors {
		errs = append(errs, map[string]string{
			"file":  filepath.Base(e.Path),
			"path":  e.Path,
			"error": e.Err.Error(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"dir":     dir,
		"added":   summary.Added,
		"updated": summary.Updated,
		"removed": summary.Removed,
		"errors":  errs,
		"version": g.cogniRegistry.Version(),
	})
}

// ── Self-Genesis handler ──

func (g *Gateway) cogniGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniGenesis == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "genesis engine not configured")
		return
	}

	var body struct {
		Description string `json:"description"`
		AutoSave    bool   `json:"auto_save"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(body.Description) == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "description is required")
		return
	}

	decl, err := g.cogniGenesis.Generate(r.Context(), body.Description)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "generation failed: "+err.Error())
		return
	}

	if body.AutoSave && g.cogniRegistry != nil {
		if err := g.cogniRegistry.Add(decl, "genesis"); err != nil {
			apperror.WriteCode(w, apperror.CodeInternal, "save failed: "+err.Error())
			return
		}
		// Persist to disk
		if g.cogniDir != "" {
			savePath := filepath.Join(g.cogniDir, decl.ID+".json")
			_ = cogni.SaveDeclaration(decl, savePath)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"declaration": decl,
		"saved":       body.AutoSave,
	})
}

// ── Workflow handlers ──

func (g *Gateway) cogniWorkflowsList(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	decl, ok := g.cogniRegistry.Get(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":        id,
		"workflows": decl.Workflows,
		"count":     len(decl.Workflows),
	})
}

func (g *Gateway) cogniWorkflowRun(w http.ResponseWriter, r *http.Request, id string, segs []string) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniWorkflowEngine == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "workflow engine not configured")
		return
	}
	decl, ok := g.cogniRegistry.Get(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni not found: "+id)
		return
	}

	wfName := ""
	if len(segs) >= 3 {
		wfName = segs[2]
	}
	if wfName == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "workflow name required: /v1/cognis/{id}/workflow/{name}")
		return
	}

	var wf *cogni.WorkflowDef
	for i := range decl.Workflows {
		if decl.Workflows[i].Name == wfName {
			wf = &decl.Workflows[i]
			break
		}
	}
	if wf == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow not found: "+wfName)
		return
	}

	var input map[string]any
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body: "+err.Error())
			return
		}
	}

	result := g.cogniWorkflowEngine.Run(r.Context(), *wf, input)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ── Experience handlers ──
