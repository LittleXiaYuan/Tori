package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/apperror"
)

//  from handlers_triggers.go 
// handleTriggers manages trigger CRUD + emit.
// GET    /v1/triggers           鈫?list all triggers
// GET    /v1/triggers?id=xxx    鈫?get one trigger
// POST   /v1/triggers           鈫?register a new trigger
// DELETE /v1/triggers?id=xxx    鈫?remove a trigger
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

//  from handlers_triggers_v2.go 
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
// Triggers V2 API 鈥?TriggerDef + TriggerRun + TriggerEvent
//
// GET    /v1/triggers/v2           鈥?list all triggers (filterable)
// GET    /v1/triggers/v2?id=xxx    鈥?get one trigger
// POST   /v1/triggers/v2           鈥?create trigger (TriggerDef)
// PUT    /v1/triggers/v2           鈥?update trigger
// DELETE /v1/triggers/v2?id=xxx    鈥?delete trigger
//
// POST   /v1/triggers/v2/emit      鈥?emit event
// GET    /v1/triggers/v2/runs      鈥?list runs
// GET    /v1/triggers/v2/events    鈥?list events
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

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

//  from handlers_cron.go 
func (g *Gateway) handleCronList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.cronMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"jobs": g.cronMgr.List()})
}

func (g *Gateway) handleCronAdd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.cronMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	var req struct {
		Name     string       `json:"name"`
		Schedule cron.Schedule `json:"schedule"`
		Payload  cron.Payload  `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "name, schedule, and payload required"})
		return
	}
	id, err := g.cronMgr.Add(req.Name, req.Schedule, req.Payload)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	job, _ := g.cronMgr.Get(id)
	json.NewEncoder(w).Encode(map[string]any{"job": job})
}

func (g *Gateway) handleCronRemove(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.cronMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "job id required"})
		return
	}
	if err := g.cronMgr.Remove(id); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

func (g *Gateway) handleCronRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.cronMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "job id required"})
		return
	}
	rec, err := g.cronMgr.RunNow(id)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"run": rec})
}

//  from handlers_session_queue.go 
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
// Session Queue API Handlers
//
// Endpoints:
//   GET  /v1/sessions/queue             鈥?list all session queues
//   GET  /v1/sessions/queue?id=xxx      鈥?get queue for session
//   POST /v1/sessions/queue/cancel      鈥?cancel a queued task
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// handleSessionQueue returns queue state for visualization.
func (g *Gateway) handleSessionQueue(w http.ResponseWriter, r *http.Request) {
	if g.queueMgr == nil {
		writeJSON(w, map[string]any{"queues": map[string]int{}})
		return
	}

	sessionID := r.URL.Query().Get("id")
	if sessionID != "" {
		snapshot := g.queueMgr.SessionSnapshot(sessionID)
		writeJSON(w, map[string]any{
			"session_id": sessionID,
			"tasks":      snapshot,
		})
		return
	}

	// All sessions
	all := g.queueMgr.AllSessions()
	writeJSON(w, map[string]any{"queues": all})
}

// handleSessionQueueCancel cancels a queued task.
func (g *Gateway) handleSessionQueueCancel(w http.ResponseWriter, r *http.Request) {
	if g.queueMgr == nil {
		http.Error(w, `{"error":"queue not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		TaskID    string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	ok := g.queueMgr.Cancel(req.SessionID, req.TaskID)
	writeJSON(w, map[string]any{"cancelled": ok})
}

