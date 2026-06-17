package knowledgepack

// import.go holds the native (de-shelled) import-url / import-repo handlers.
// They talk to the knowledge store directly and use the knowledge domain layer
// (internal/agentcore/knowledge) for transport-free web/repo logic. The only
// gateway coupling is the narrow KnowledgeGateway interface: an SSRF-safe fetch
// (FetchImportPage), the output dir (OutputDir) and tenant resolution (TenantOf).

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/pkg/safego"
)

// handleImportURL imports a single URL (optionally crawling DeepWiki children)
// natively: it fetches via the gateway's SSRF-safe fetcher, ingests through the
// store and triggers an async reindex.
func (h *Handler) handleImportURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		URL           string `json:"url"`
		Name          string `json:"name"`
		CrawlChildren bool   `json:"crawl_children"`
		MaxPages      int    `json:"max_pages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "url required"})
		return
	}

	if req.MaxPages <= 0 {
		req.MaxPages = 5
	}
	if req.MaxPages > 20 {
		req.MaxPages = 20
	}

	page, err := h.gateway.FetchImportPage(strings.TrimSpace(req.URL), strings.TrimSpace(req.Name))
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	imported := make([]*knowledge.Source, 0, req.MaxPages)
	src, err := h.store.IngestURL(page.Name, page.URL, page.Content)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	imported = append(imported, src)

	if req.CrawlChildren {
		for _, childURL := range knowledge.ExtractChildLinks(page.URL, page.RawHTML, req.MaxPages-1) {
			childPage, childErr := h.gateway.FetchImportPage(childURL, "")
			if childErr != nil {
				slog.Warn("knowledge: import child url failed", "url", childURL, "err", childErr)
				continue
			}
			childSrc, childErr := h.store.IngestURL(childPage.Name, childPage.URL, childPage.Content)
			if childErr != nil {
				slog.Warn("knowledge: ingest child url failed", "url", childURL, "err", childErr)
				continue
			}
			imported = append(imported, childSrc)
		}
	}

	safego.Go("knowledge-reindex", func() {
		if err := h.store.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after import-url failed", "err", err)
		}
	})

	_ = json.NewEncoder(w).Encode(map[string]any{
		"source":   src,
		"sources":  imported,
		"imported": len(imported),
		"tree":     knowledge.BuildImportTree(page, imported),
		"stats":    h.store.Stats(),
	})
}

// handleImportRepo imports a local repository / code directory natively. The
// resolved path must sit under an allowed import root (see
// knowledge.ResolveRepoPath) to prevent arbitrary local-file exfiltration.
func (h *Handler) handleImportRepo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		Path     string `json:"path"`
		MaxFiles int    `json:"max_files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Path) == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "path required"})
		return
	}
	userPath := strings.TrimSpace(req.Path)
	resolvedPath, err := knowledge.ResolveRepoPath(h.gateway.OutputDir(), userPath)
	if err != nil {
		slog.Warn("knowledge: import-repo rejected",
			"tenant", h.gateway.TenantOf(r.Context()),
			"path", userPath,
			"err", err)
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	src, err := h.store.IngestDirectory(resolvedPath, req.MaxFiles)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	safego.Go("knowledge-reindex", func() {
		if err := h.store.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after import-repo failed", "err", err)
		}
	})
	_ = json.NewEncoder(w).Encode(map[string]any{"source": src, "stats": h.store.Stats()})
}

// writeJSONStatus writes a JSON body with an explicit HTTP status code.
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
