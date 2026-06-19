package gateway

import (
	"encoding/json"
	"net/http"
	"path/filepath"
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
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni collection is owned by cogni-kernel pack")
	case path == "runtime/pack-state":
		if g.cogniKernelRuntimeState == nil {
			apperror.WriteCode(w, apperror.CodeInternal, "cogni runtime state reporter not configured")
			return
		}
		g.cogniKernelRuntimeState(w, r)
	case path == "reload":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni reload is owned by cogni-kernel pack")
	case path == "generate":
		g.cogniGenerate(w, r)
	case path == "export":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni export is owned by cogni-kernel pack")
	case path == "import":
		apperror.WriteCode(w, apperror.CodeNotFound, "cogni import is owned by cogni-kernel pack")
	case path == "evolution":
		g.cogniEvolutionList(w, r)
	default:
		segs := strings.SplitN(path, "/", 3)
		id := segs[0]
		switch {
		case len(segs) == 1:
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni declaration route is owned by cogni-kernel pack")
		case len(segs) == 2 && segs[1] == "enable":
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni enable route is owned by cogni-kernel pack")
		case len(segs) == 2 && segs[1] == "disable":
			apperror.WriteCode(w, apperror.CodeNotFound, "cogni disable route is owned by cogni-kernel pack")
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

// ── Experience handlers ──
