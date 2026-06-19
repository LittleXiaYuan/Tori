package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"yunque-agent/internal/agentcore/audit"
)

// RBAC routes were de-shelled into the RBAC pack (internal/packs/rbac). The
// gateway only exposes RBACEnforcer(), RequireAuth(), RequireAdmin() and
// TenantOf() as narrow pack accessors.

// from handlers_audit.go
func (g *Gateway) handleAuditTail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.auditChain == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
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

	// Optional filters
	typ := audit.EventType(r.URL.Query().Get("type"))
	actor := r.URL.Query().Get("actor")

	var records []audit.Record
	if typ != "" || actor != "" {
		records = g.auditChain.Search(typ, actor, n)
	} else {
		records = g.auditChain.Tail(n)
	}
	json.NewEncoder(w).Encode(map[string]any{"records": records, "count": len(records)})
}

func (g *Gateway) handleAuditVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.auditChain == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
		return
	}
	idx := g.auditChain.Verify()
	result := map[string]any{
		"valid":        idx == -1,
		"checked":      g.auditChain.Len(),
		"chain_length": g.auditChain.Len(),
	}
	if idx != -1 {
		result["broken_at"] = idx
		result["tampered_at"] = idx
	}
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.auditChain == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "audit not configured"})
		return
	}
	json.NewEncoder(w).Encode(g.auditChain.Stats())
}

// Execution trace APIs were de-shelled into the trace pack
// (internal/packs/trace). The gateway only exposes EventTrail().
