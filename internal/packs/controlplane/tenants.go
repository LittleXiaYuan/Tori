package controlplanepack

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
)

func (h *Handler) handleTenants(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleCreateTenant(w, r)
	case http.MethodGet:
		h.handleListTenants(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	tenants := h.gateway.TenantManager()
	if tenants == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "tenant system not available")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	created := tenants.Register(req.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (h *Handler) handleListTenants(w http.ResponseWriter, r *http.Request) {
	tenants := h.gateway.TenantManager()
	if tenants == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "tenant system not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	list := tenants.List()
	_ = json.NewEncoder(w).Encode(map[string]any{"tenants": list, "count": len(list)})
}
