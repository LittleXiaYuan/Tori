package knowledgepack

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/pkg/safego"
)

// handleUpload ingests an uploaded text/markdown/document file natively. The
// pack owns the HTTP surface; the gateway only supplies the configured document
// parser through KnowledgeGateway.DocumentParser().
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file field required (max 10MB)"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "read file failed"})
		return
	}

	result, err := knowledge.IngestUpload(r.Context(), h.store, h.gateway.DocumentParser(), header.Filename, data)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	safego.Go("knowledge-reindex", func() {
		if err := h.store.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after upload failed", "err", err)
		}
	})

	resp := map[string]any{"source": result.Source, "stats": h.store.Stats()}
	if result.Parse != nil {
		resp["parse"] = result.Parse
	}
	_ = json.NewEncoder(w).Encode(resp)
}
