package workflowapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/gwshared"
	"yunque-agent/pkg/safego"
)

// Handler serves workflow engine HTTP endpoints.
type Handler struct {
	Store  workflow.Store
	Engine *workflow.Engine
}

// RegisterRoutes mounts all /v1/workflows/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	mux.HandleFunc("/v1/workflows", auth(h.handleRouteSwitch))
	mux.HandleFunc("/v1/workflows/run", auth(h.handleRun))
	mux.HandleFunc("/v1/workflows/instances", auth(h.handleInstances))
	mux.HandleFunc("/v1/workflows/cancel", auth(h.handleCancel))
}

func (h *Handler) handleRouteSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleList(w, r)
	case http.MethodPost:
		h.handleSave(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/DELETE only")
	}
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id != "" {
		def, err := h.Store.GetDefinition(id)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		json.NewEncoder(w).Encode(def)
		return
	}
	defs, err := h.Store.ListDefinitions("")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	if defs == nil {
		defs = []*workflow.Definition{}
	}
	json.NewEncoder(w).Encode(map[string]any{"workflows": defs, "total": len(defs)})
}

func (h *Handler) handleSave(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	var def workflow.Definition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if def.Name == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
		return
	}
	def.TenantID = gwshared.TenantFromCtx(r.Context())
	if err := h.Store.SaveDefinition(&def); err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(def)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id query param required")
		return
	}
	if err := h.Store.DeleteDefinition(id); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil || h.Engine == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	var req struct {
		DefinitionID string         `json:"definition_id"`
		Variables    map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.DefinitionID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "definition_id is required")
		return
	}
	tenantID := gwshared.TenantFromCtx(r.Context())
	inst, err := h.Store.CreateInstance(req.DefinitionID, tenantID, req.Variables)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	engine := h.Engine
	safego.Go("workflow-run-"+inst.ID, func() {
		if err := engine.Run(context.Background(), inst.ID); err != nil {
			slog.Warn("workflow execution failed", "instance", inst.ID, "err", err)
		}
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":      "accepted",
		"instance_id": inst.ID,
		"instance":    inst,
	})
}

func (h *Handler) handleInstances(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id != "" {
		inst, err := h.Store.GetInstance(id)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		json.NewEncoder(w).Encode(inst)
		return
	}
	tenantID := gwshared.TenantFromCtx(r.Context())
	insts, err := h.Store.ListInstances(tenantID, 50)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	if insts == nil {
		insts = []*workflow.Instance{}
	}
	json.NewEncoder(w).Encode(map[string]any{"instances": insts, "total": len(insts)})
}

func (h *Handler) handleCancel(w http.ResponseWriter, r *http.Request) {
	if h.Engine == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.InstanceID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "instance_id is required")
		return
	}
	if !h.Engine.Cancel(req.InstanceID) {
		apperror.WriteCode(w, apperror.CodeNotFound, "instance not running")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "cancelling",
		"instance_id": req.InstanceID,
	})
}
