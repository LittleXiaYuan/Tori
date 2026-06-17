package controlplanepack

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"yunque-agent/internal/agentcore/audit"
)

func (h *Handler) handleAuditTail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	chain := h.gateway.AuditChain()
	if chain == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
		return
	}
	n := 20
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	if n > 200 {
		n = 200
	}
	typ := audit.EventType(r.URL.Query().Get("type"))
	actor := r.URL.Query().Get("actor")
	var records []audit.Record
	if typ != "" || actor != "" {
		records = chain.Search(typ, actor, n)
	} else {
		records = chain.Tail(n)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"records": records, "count": len(records)})
}

func (h *Handler) handleAuditVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	chain := h.gateway.AuditChain()
	if chain == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
		return
	}
	idx := chain.Verify()
	result := map[string]any{
		"valid":        idx == -1,
		"checked":      chain.Len(),
		"chain_length": chain.Len(),
	}
	if idx != -1 {
		result["broken_at"] = idx
		result["tampered_at"] = idx
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	chain := h.gateway.AuditChain()
	if chain == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(chain.Stats())
}

func (h *Handler) handleAuditTrail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	trail := h.gateway.AuditTrail()
	if trail == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": []any{}, "count": 0})
		return
	}
	date := time.Now()
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			date = t
		}
	}
	entries := trail.Query(date, r.URL.Query().Get("type"))
	_ = json.NewEncoder(w).Encode(map[string]any{"entries": entries, "count": len(entries)})
}
