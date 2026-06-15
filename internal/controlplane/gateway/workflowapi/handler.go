package workflowapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/gwshared"
	"yunque-agent/pkg/safego"
)

// Handler serves workflow engine HTTP endpoints.
type Handler struct {
	Store   workflow.Store
	Engine  *workflow.Engine
	LLMCall workflow.LLMCallFunc

	mu sync.RWMutex
}

// RegisterRoutes mounts all /v1/workflows/* endpoints. Retained for callers that
// own a raw mux (e.g. workflowapi's own tests); the gateway now mounts these
// through the work pack via RouteSpecs so workflow lives in the task platform.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	for _, rt := range h.RouteSpecs() {
		mux.HandleFunc(rt.Path, auth(rt.Handler))
	}
}

// Route is one workflow HTTP endpoint (path + handler), used to embed the
// workflow surface into the work pack (internal/packs/work) instead of mounting
// it as a detached gateway sub-package.
type Route struct {
	Path    string
	Handler http.HandlerFunc
}

// RouteSpecs returns the workflow endpoints so another pack/mux can own them.
func (h *Handler) RouteSpecs() []Route {
	return []Route{
		{Path: "/v1/workflows", Handler: h.handleRouteSwitch},
		{Path: "/v1/workflows/generate", Handler: h.handleGenerate},
		{Path: "/v1/workflows/run", Handler: h.handleRun},
		{Path: "/v1/workflows/instances", Handler: h.handleInstances},
		{Path: "/v1/workflows/cancel", Handler: h.handleCancel},
	}
}

// SetStore updates the backing workflow store after the handler has already
// been mounted. The gateway constructs routes before every runtime subsystem is
// available, so this keeps /v1/workflows live once init_task_engine finishes.
func (h *Handler) SetStore(store workflow.Store) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Store = store
}

// SetEngine updates the execution engine after route registration.
func (h *Handler) SetEngine(engine *workflow.Engine) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Engine = engine
}

// SetLLMCall updates the optional LLM generator hook after route registration.
func (h *Handler) SetLLMCall(fn workflow.LLMCallFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.LLMCall = fn
}

func (h *Handler) snapshot() (workflow.Store, *workflow.Engine, workflow.LLMCallFunc) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Store, h.Engine, h.LLMCall
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

func (h *Handler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	store, _, llmCall := h.snapshot()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	var req struct {
		Requirement string `json:"requirement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	result, err := workflow.GenerateDefinition(r.Context(), req.Requirement, workflow.GeneratorOptions{
		TenantID: gwshared.TenantFromCtx(r.Context()),
		LLMCall:  llmCall,
	})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	if err := store.SaveDefinition(result.Definition); err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":           true,
		"workflow":     result.Definition,
		"generated_by": string(result.Source),
		"message":      result.Message,
	})
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	store, _, _ := h.snapshot()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id != "" {
		def, err := store.GetDefinition(id)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		json.NewEncoder(w).Encode(def)
		return
	}
	defs, err := store.ListDefinitions("")
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
	store, _, _ := h.snapshot()
	if store == nil {
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
	if err := store.SaveDefinition(&def); err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(def)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	store, _, _ := h.snapshot()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id query param required")
		return
	}
	if err := store.DeleteDefinition(id); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	store, engine, _ := h.snapshot()
	if store == nil || engine == nil {
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
	inst, err := store.CreateInstance(req.DefinitionID, tenantID, req.Variables)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
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
	store, _, _ := h.snapshot()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id != "" {
		inst, err := store.GetInstance(id)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		json.NewEncoder(w).Encode(inst)
		return
	}
	tenantID := gwshared.TenantFromCtx(r.Context())
	insts, err := store.ListInstances(tenantID, 50)
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
	_, engine, _ := h.snapshot()
	if engine == nil {
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
	if !engine.Cancel(req.InstanceID) {
		apperror.WriteCode(w, apperror.CodeNotFound, "instance not running")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "cancelling",
		"instance_id": req.InstanceID,
	})
}
