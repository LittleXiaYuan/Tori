package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/apperror"
)

// handleTriggers manages trigger CRUD + emit.
// GET    /v1/triggers           → list all triggers
// GET    /v1/triggers?id=xxx    → get one trigger
// POST   /v1/triggers           → register a new trigger
// DELETE /v1/triggers?id=xxx    → remove a trigger
func (g *Gateway) handleTriggers(w http.ResponseWriter, r *http.Request) {
	if g.triggerRT == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger runtime not available")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			t, ok := g.triggerRT.Get(id)
			if !ok {
				apperror.WriteCode(w, apperror.CodeNotFound, "trigger not found")
				return
			}
			json.NewEncoder(w).Encode(t)
			return
		}
		triggers := g.triggerRT.List()
		json.NewEncoder(w).Encode(map[string]any{"triggers": triggers, "total": len(triggers)})

	case http.MethodPost:
		var t trigger.Trigger
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if t.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		id := g.triggerRT.Register(t)
		t.ID = id
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(t)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		if !g.triggerRT.Remove(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "trigger not found")
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"deleted": id})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/DELETE only")
	}
}

// handleTriggerEmit emits a system event to the trigger runtime.
// POST /v1/triggers/emit { "event": "task_completed", "text": "...", "data": {...} }
func (g *Gateway) handleTriggerEmit(w http.ResponseWriter, r *http.Request) {
	if g.triggerRT == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger runtime not available")
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var payload trigger.EventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if payload.Event == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "event is required")
		return
	}

	g.triggerRT.Emit(r.Context(), payload)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "emitted", "event": string(payload.Event)})
}
