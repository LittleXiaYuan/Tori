package gateway

import (
	"encoding/json"
	"net/http"
)

//  from handlers_triggers.go 
// Trigger HTTP handlers (legacy Runtime + unified Manager) were de-shelled into
// the triggers pack (internal/packs/triggers); the subsystems are reached there
// via the gateway's TriggerRuntime()/TriggerManager() accessors.

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


// Cron HTTP handlers were de-shelled into the cron pack (internal/packs/cron);
// the cron manager is reached there via the gateway's CronManager() accessor.

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

