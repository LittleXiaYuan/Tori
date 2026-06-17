package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/appdir"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/sandbox"
)

func (g *Gateway) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	t := g.tenants.Register(req.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func (g *Gateway) handleListTenants(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	list := g.tenants.List()
	json.NewEncoder(w).Encode(map[string]any{"tenants": list, "count": len(list)})
}

func (g *Gateway) handleTokenGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	t := g.tenants.ByAPIKey(apiKey)
	if t == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid api key"})
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Role == "" {
		req.Role = "user"
	}

	allowedRoles := map[string]bool{"user": true, "viewer": true}
	if !allowedRoles[req.Role] {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "role not allowed via API key exchange"})
		return
	}

	if g.jwtCfg == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "jwt not configured"})
		return
	}

	token, err := GenerateJWT(*g.jwtCfg, t.ID, req.Role)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token, "type": "Bearer"})
}

func (g *Gateway) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeMissingField, "file field required")
		return
	}
	defer file.Close()

	filename := filepath.Base(strings.TrimSpace(header.Filename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid filename")
		return
	}

	ext := strings.ToLower(filepath.Ext(filename))
	blockedExts := map[string]bool{
		".exe": true, ".bat": true, ".cmd": true, ".com": true, ".msi": true,
		".sh": true, ".bash": true, ".ps1": true, ".vbs": true, ".wsf": true,
		".scr": true, ".pif": true, ".dll": true, ".so": true, ".dylib": true,
	}
	if blockedExts[ext] {
		apperror.WriteCode(w, apperror.CodeBadRequest, "file type not allowed: "+ext)
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "read file failed", err))
		return
	}

	tid := tenantFromCtx(r.Context())
	uploadTenant := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' {
			return '_'
		}
		return r
	}, tid)
	if uploadTenant == "" {
		uploadTenant = "default"
	}
	sb, sbErr := sandbox.New(appdir.Sub("uploads", uploadTenant), sandbox.DefaultPolicy())
	if sbErr != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "sandbox init failed", sbErr))
		return
	}
	sb.WriteFile(filename, string(content))
	// savedPath is kept for server-side consumers (e.g. AnalysisToActions
	// below) that need the real on-disk location. We deliberately surface
	// only `relPath` over the wire so authenticated callers do not learn the
	// agent's host filesystem layout, which would simplify chained attacks
	// (path traversal crafted against a known sandbox root, social
	// engineering with a real username in the path, etc.).
	savedPath := filepath.Join(sb.WorkDir(), filename)
	relPath := filename

	slog.Info("file uploaded", "tenant", tid, "name", filename, "size", len(content))

	resp := map[string]any{
		"filename": filename,
		"size":     len(content),
		"path":     relPath,
	}

	localParse := TryParseFileResult(filename, content)
	snippet := localParse.Preview
	if parseMeta := fileParseMetadata(localParse, 6000); parseMeta != nil {
		resp["parse"] = parseMeta
	}
	if knowledge.IsMinerUSupportedExt(ext) && g.documentParser != nil && g.documentParser.Enabled() {
		if parsed, perr := knowledge.ParseFileWithMinerU(r.Context(), g.documentParser, filename, content); perr != nil {
			slog.Warn("upload MinerU parse failed", "name", filename, "err", perr)
		} else {
			snippet = parsed.Markdown
			resp["parse"] = parsed.Parse
		}
	}
	if g.planner != nil {
		analysisCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if analysis, aerr := g.planner.AnalyzeUploadedFile(analysisCtx, filename, snippet); aerr != nil {
			slog.Debug("upload template analysis skipped", "err", aerr)
		} else {
			actions := planner.AnalysisToActions(savedPath, analysis)
			if len(actions) > 0 {
				resp["analysis"] = analysis
				resp["actions"] = actions
				if rich := RenderAgentActions(actions); rich != nil {
					resp["rich"] = json.RawMessage(rich.ToJSON())
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
