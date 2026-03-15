package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/apperror"
)

// ──────────────────────────────────────────────
// Triggers V2 API — TriggerDef + TriggerRun + TriggerEvent
//
// GET    /v1/triggers/v2           — list all triggers (filterable)
// GET    /v1/triggers/v2?id=xxx    — get one trigger
// POST   /v1/triggers/v2           — create trigger (TriggerDef)
// PUT    /v1/triggers/v2           — update trigger
// DELETE /v1/triggers/v2?id=xxx    — delete trigger
//
// POST   /v1/triggers/v2/emit      — emit event
// GET    /v1/triggers/v2/runs      — list runs
// GET    /v1/triggers/v2/events    — list events
// ──────────────────────────────────────────────

func (g *Gateway) handleTriggersV2(w http.ResponseWriter, r *http.Request) {
	if g.triggerMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger manager not available")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			t, ok := g.triggerMgr.Get(id)
			if !ok {
				apperror.WriteCode(w, apperror.CodeNotFound, "trigger not found")
				return
			}
			json.NewEncoder(w).Encode(t)
			return
		}

		// List with filters
		tenantID := r.URL.Query().Get("tenant_id")
		triggerType := r.URL.Query().Get("type")
		status := r.URL.Query().Get("status")

		triggers := g.triggerMgr.List(tenantID, func(t *trigger.TriggerDef) bool {
			if triggerType != "" && string(t.Type) != triggerType {
				return false
			}
			if status != "" && string(t.Status) != status {
				return false
			}
			return true
		})

		json.NewEncoder(w).Encode(map[string]any{
			"triggers": triggers,
			"total":    len(triggers),
		})

	case http.MethodPost:
		var t trigger.TriggerDef
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if t.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		if t.TenantID == "" {
			t.TenantID = "default"
		}
		if len(t.Actions) == 0 {
			apperror.WriteCode(w, apperror.CodeBadRequest, "at least one action is required")
			return
		}

		if err := g.triggerMgr.Create(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeInternal, err.Error())
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(t)

	case http.MethodPut:
		var t trigger.TriggerDef
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if t.ID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}

		if err := g.triggerMgr.Update(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}

		json.NewEncoder(w).Encode(t)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}

		if err := g.triggerMgr.Delete(id); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"deleted": id})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/PUT/DELETE only")
	}
}

func (g *Gateway) handleTriggersV2Emit(w http.ResponseWriter, r *http.Request) {
	if g.triggerMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger manager not available")
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var payload trigger.EventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if payload.Event == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "event is required")
		return
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now()
	}

	g.triggerMgr.Emit(r.Context(), payload)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "emitted",
		"event":  string(payload.Event),
	})
}

func (g *Gateway) handleTriggersV2Runs(w http.ResponseWriter, r *http.Request) {
	if g.triggerMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger manager not available")
		return
	}
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}

	triggerID := r.URL.Query().Get("trigger_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	runs := g.triggerMgr.GetRuns(triggerID, limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"runs":  runs,
		"total": len(runs),
	})
}

func (g *Gateway) handleTriggersV2Events(w http.ResponseWriter, r *http.Request) {
	if g.triggerMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger manager not available")
		return
	}
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}

	triggerID := r.URL.Query().Get("trigger_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	events := g.triggerMgr.GetEvents(triggerID, limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"events": events,
		"total":  len(events),
	})
}
