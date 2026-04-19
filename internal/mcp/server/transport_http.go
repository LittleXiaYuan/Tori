package server

import (
	"io"
	"log/slog"
	"net/http"
)

// HTTPHandler returns an http.HandlerFunc that implements the MCP
// Streamable HTTP transport. Each POST carries a single JSON-RPC 2.0
// request; the response is returned synchronously in the body.
func HTTPHandler(srv *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","server":"yunque-mcp-dispatch"}`))
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB max
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		resp, err := srv.HandleRequest(r.Context(), body)
		if err != nil {
			slog.Error("mcp server handler error", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}
}
