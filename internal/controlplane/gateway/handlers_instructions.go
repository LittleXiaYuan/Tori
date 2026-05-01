package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/instruction"
	"yunque-agent/internal/apperror"
)

func (g *Gateway) handleInstructions(w http.ResponseWriter, r *http.Request) {
	if g.instructionStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "instruction store not configured")
		return
	}
	ctx := r.Context()
	tenantID := tenantFromCtx(ctx)

	switch r.Method {
	case http.MethodGet:
		list, err := g.instructionStore.List(ctx, tenantID)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeInternal, err.Error())
			return
		}
		if category := r.URL.Query().Get("category"); category != "" {
			filtered := list[:0]
			for _, inst := range list {
				if inst.Category == category {
					filtered = append(filtered, inst)
				}
			}
			list = filtered
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"instructions": list,
			"total":        len(list),
		})

	case http.MethodPost:
		var req instruction.UserInstruction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
			return
		}
		created, err := g.instructionStore.Create(ctx, tenantID, req)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)

	case http.MethodPut:
		var req instruction.UserInstruction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
			return
		}
		if req.ID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "instruction_id is required")
			return
		}
		if err := g.instructionStore.Update(ctx, tenantID, req); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id query parameter required")
			return
		}
		if err := g.instructionStore.Delete(ctx, tenantID, id); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (g *Gateway) handleInstructionsReorder(w http.ResponseWriter, r *http.Request) {
	if g.instructionStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "instruction store not configured")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	tenantID := tenantFromCtx(ctx)

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
		return
	}
	if len(req.IDs) == 0 {
		apperror.WriteCode(w, apperror.CodeBadRequest, "ids array cannot be empty")
		return
	}
	if err := g.instructionStore.Reorder(ctx, tenantID, req.IDs); err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reordered"})
}
