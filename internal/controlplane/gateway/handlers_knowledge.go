package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/pkg/safego"
)

func (g *Gateway) handleKBSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "query parameter 'q' required"})
		return
	}
	limit := 10
	if n := r.URL.Query().Get("n"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 50 {
		limit = 50
	}
	fileFilter := r.URL.Query().Get("file")
	langFilter := r.URL.Query().Get("lang")
	chunks := g.knowledgeStore.SearchFiltered(query, limit, fileFilter, langFilter)
	json.NewEncoder(w).Encode(map[string]any{"chunks": chunks, "count": len(chunks)})
}

func (g *Gateway) handleKBSources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"sources": g.knowledgeStore.Sources()})
}

func (g *Gateway) handleKBStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	json.NewEncoder(w).Encode(g.knowledgeStore.Stats())
}

func (g *Gateway) handleKBIngest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		Name    string `json:"name"`
		Trigger string `json:"trigger"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "name and content required"})
		return
	}
	if req.Name == "" {
		req.Name = "inline-text"
	}

	var src *knowledge.Source
	var err error
	if req.Trigger != "" {
		src, err = g.knowledgeStore.IngestStructured(req.Name, req.Trigger, req.Content)
	} else {
		src, err = g.knowledgeStore.IngestText(req.Name, req.Content)
	}
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after ingest failed", "err", err)
		}
	})

	json.NewEncoder(w).Encode(map[string]any{"source": src, "stats": g.knowledgeStore.Stats()})
}

func (g *Gateway) handleKBUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Trigger string `json:"trigger"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "id required"})
		return
	}

	src, err := g.knowledgeStore.UpdateSource(req.ID, req.Name, req.Trigger, req.Content)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after update failed", "err", err)
		}
	})

	json.NewEncoder(w).Encode(map[string]any{"source": src, "stats": g.knowledgeStore.Stats()})
}

func (g *Gateway) handleKBDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	sourceID := r.URL.Query().Get("id")
	if sourceID == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "source id required"})
		return
	}
	ok := g.knowledgeStore.RemoveSource(sourceID)
	if !ok {
		json.NewEncoder(w).Encode(map[string]string{"error": "source not found"})
		return
	}

	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after delete failed", "err", err)
		}
	})

	json.NewEncoder(w).Encode(map[string]any{"deleted": sourceID, "stats": g.knowledgeStore.Stats()})
}
