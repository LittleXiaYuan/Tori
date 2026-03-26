package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/apperror"
)

// handleWorkflowRouteSwitch dispatches /v1/workflows by method.
func (g *Gateway) handleWorkflowRouteSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.handleWorkflowList(w, r)
	case http.MethodPost:
		g.handleWorkflowSave(w, r)
	case http.MethodDelete:
		g.handleWorkflowDelete(w, r)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/DELETE only")
	}
}

// handleWorkflowList lists workflow definitions.
// GET /v1/workflows
// GET /v1/workflows?id=xxx → get one
func (g *Gateway) handleWorkflowList(w http.ResponseWriter, r *http.Request) {
	if g.workflowStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id != "" {
		def, err := g.workflowStore.GetDefinition(id)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		json.NewEncoder(w).Encode(def)
		return
	}

	tenantID := tenantFromCtx(r.Context())
	// Workflow management is an admin panel — show all workflows.
	// NL2Workflow-generated definitions may have a different tenant than
	// the JWT-authenticated user; filtering would hide them.
	_ = tenantID
	defs, err := g.workflowStore.ListDefinitions("")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	if defs == nil {
		defs = []*workflow.Definition{}
	}
	json.NewEncoder(w).Encode(map[string]any{"workflows": defs, "total": len(defs)})
}

// handleWorkflowSave creates or updates a workflow definition.
// POST /v1/workflows
func (g *Gateway) handleWorkflowSave(w http.ResponseWriter, r *http.Request) {
	if g.workflowStore == nil {
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
	def.TenantID = tenantFromCtx(r.Context())

	if err := g.workflowStore.SaveDefinition(&def); err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(def)
}

// handleWorkflowDelete deletes a workflow definition.
// DELETE /v1/workflows?id=xxx
func (g *Gateway) handleWorkflowDelete(w http.ResponseWriter, r *http.Request) {
	if g.workflowStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id query param required")
		return
	}
	if err := g.workflowStore.DeleteDefinition(id); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

// handleWorkflowRun creates an instance from a definition and starts execution.
// POST /v1/workflows/run { "definition_id": "xxx", "variables": {...} }
func (g *Gateway) handleWorkflowRun(w http.ResponseWriter, r *http.Request) {
	if g.workflowStore == nil || g.workflowEngine == nil {
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

	tenantID := tenantFromCtx(r.Context())
	inst, err := g.workflowStore.CreateInstance(req.DefinitionID, tenantID, req.Variables)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	// Run async
	go func() {
		if err := g.workflowEngine.Run(context.Background(), inst.ID); err != nil {
			slog.Warn("workflow execution failed", "instance", inst.ID, "err", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":      "accepted",
		"instance_id": inst.ID,
		"instance":    inst,
	})
}

// handleWorkflowInstances lists workflow instances.
// GET /v1/workflows/instances
// GET /v1/workflows/instances?id=xxx → get one
func (g *Gateway) handleWorkflowInstances(w http.ResponseWriter, r *http.Request) {
	if g.workflowStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "workflow engine not available")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id != "" {
		inst, err := g.workflowStore.GetInstance(id)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		json.NewEncoder(w).Encode(inst)
		return
	}

	tenantID := tenantFromCtx(r.Context())
	insts, err := g.workflowStore.ListInstances(tenantID, 50)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	if insts == nil {
		insts = []*workflow.Instance{}
	}
	json.NewEncoder(w).Encode(map[string]any{"instances": insts, "total": len(insts)})
}

// handleWorkflowCancel cancels a running workflow instance.
// POST /v1/workflows/cancel { "instance_id": "xxx" }
func (g *Gateway) handleWorkflowCancel(w http.ResponseWriter, r *http.Request) {
	if g.workflowEngine == nil {
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

	if !g.workflowEngine.Cancel(req.InstanceID) {
		apperror.WriteCode(w, apperror.CodeNotFound, "instance not running")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "cancelling",
		"instance_id": req.InstanceID,
	})
}
