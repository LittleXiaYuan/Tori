package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/apperror"
)

// handleTaskCreate creates a new task.
// POST /v1/tasks
func (g *Gateway) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	if g.taskStore == nil || g.taskRunner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	var req task.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	req.TenantID = tenantFromCtx(r.Context())

	t, err := g.taskStore.Create(req)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

// handleTaskList lists tasks or gets a single task by id.
// GET /v1/tasks          → list all
// GET /v1/tasks?id=xxx   → get one
func (g *Gateway) handleTaskList(w http.ResponseWriter, r *http.Request) {
	if g.taskStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id != "" {
		t, ok := g.taskStore.Get(id)
		if !ok {
			apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
			return
		}
		json.NewEncoder(w).Encode(t)
		return
	}

	tenantID := tenantFromCtx(r.Context())
	tasks := g.taskStore.List(tenantID, 50)
	json.NewEncoder(w).Encode(tasks)
}

// handleTaskRun starts executing a task.
// POST /v1/tasks/run  { "id": "xxx" }
func (g *Gateway) handleTaskRun(w http.ResponseWriter, r *http.Request) {
	if g.taskStore == nil || g.taskRunner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	t, ok := g.taskStore.Get(req.ID)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	if t.IsTerminal() {
		apperror.WriteCode(w, apperror.CodeBadRequest, "task already finished")
		return
	}

	// Run async — return immediately, client polls for status
	go func() {
		ctx := context.Background()
		if err := g.taskRunner.Run(ctx, req.ID); err != nil {
			slog.Warn("task execution failed", "task", req.ID, "err", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"task_id": req.ID,
	})
}

// handleTaskCancel cancels a running task.
// POST /v1/tasks/cancel  { "id": "xxx" }
func (g *Gateway) handleTaskCancel(w http.ResponseWriter, r *http.Request) {
	if g.taskStore == nil || g.taskRunner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	t, ok := g.taskStore.Get(req.ID)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	if t.IsTerminal() {
		apperror.WriteCode(w, apperror.CodeBadRequest, "task already finished")
		return
	}

	if !g.taskRunner.Cancel(req.ID) {
		apperror.WriteCode(w, apperror.CodeBadRequest, "task is not currently running")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "cancelling",
		"task_id": req.ID,
	})
}

// handleTaskPause pauses a running task after the current step completes.
// POST /v1/tasks/pause  { "id": "xxx" }
func (g *Gateway) handleTaskPause(w http.ResponseWriter, r *http.Request) {
	if g.taskStore == nil || g.taskRunner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	if !g.taskRunner.Pause(req.ID) {
		apperror.WriteCode(w, apperror.CodeBadRequest, "task is not currently running")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "pausing",
		"task_id": req.ID,
	})
}

// handleTaskResume resumes a paused, interrupted, or failed task.
// POST /v1/tasks/resume  { "id": "xxx" }
func (g *Gateway) handleTaskResume(w http.ResponseWriter, r *http.Request) {
	if g.taskStore == nil || g.taskRunner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	t, ok := g.taskStore.Get(req.ID)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	if !t.IsResumable() {
		apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("task in state %s is not resumable", t.Status))
		return
	}

	// Resume async
	go func() {
		ctx := context.Background()
		if err := g.taskRunner.Resume(ctx, req.ID); err != nil {
			slog.Warn("task resume failed", "task", req.ID, "err", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "resuming",
		"task_id": req.ID,
	})
}

// handleTaskRestart restarts a task from scratch (re-plans and re-executes).
// POST /v1/tasks/restart  { "id": "xxx" }
func (g *Gateway) handleTaskRestart(w http.ResponseWriter, r *http.Request) {
	if g.taskStore == nil || g.taskRunner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	t, ok := g.taskStore.Get(req.ID)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	if !t.IsTerminal() && !t.IsResumable() {
		apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("task in state %s cannot be restarted", t.Status))
		return
	}

	// Restart async
	go func() {
		ctx := context.Background()
		if err := g.taskRunner.Restart(ctx, req.ID); err != nil {
			slog.Warn("task restart failed", "task", req.ID, "err", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "restarting",
		"task_id": req.ID,
	})
}

// handleTaskDelete deletes a task.
// DELETE /v1/tasks?id=xxx
func (g *Gateway) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	if g.taskStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id query param required")
		return
	}
	if !g.taskStore.Delete(id) {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

// handleGaps returns capability gap records and stats.
// GET /v1/tasks/gaps              → all unresolved gaps
// GET /v1/tasks/gaps?type=xxx     → filter by gap type
// GET /v1/tasks/gaps?stats=true   → return stats only
// POST /v1/tasks/gaps/resolve { "id": "gap-1" } → mark resolved
func (g *Gateway) handleGaps(w http.ResponseWriter, r *http.Request) {
	if g.gapAnalyzer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "gap analyzer not available")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if r.URL.Query().Get("stats") == "true" {
		json.NewEncoder(w).Encode(g.gapAnalyzer.Stats())
		return
	}

	gapType := task.GapType(r.URL.Query().Get("type"))
	records := g.gapAnalyzer.Records(gapType, true)
	if records == nil {
		records = []task.GapRecord{} // return empty array, not null
	}
	json.NewEncoder(w).Encode(records)
}

// handleGapResolve marks a gap as resolved.
// POST /v1/tasks/gaps/resolve { "id": "gap-1" }
func (g *Gateway) handleGapResolve(w http.ResponseWriter, r *http.Request) {
	if g.gapAnalyzer == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "gap analyzer not available")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}

	if !g.gapAnalyzer.Resolve(req.ID) {
		apperror.WriteCode(w, apperror.CodeNotFound, "gap not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"resolved": req.ID})
}

// handleTaskWorkingMemory returns the working memory for a task.
// GET /v1/tasks/memory?id=xxx
func (g *Gateway) handleTaskWorkingMemory(w http.ResponseWriter, r *http.Request) {
	if g.workMemMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "working memory not available")
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	wm := g.workMemMgr.Get(taskID)
	if wm == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "no working memory for this task")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wm)
}

// handleTemplates handles CRUD for task templates.
// GET /v1/tasks/templates — list all or get by id
// POST /v1/tasks/templates — create a template
// DELETE /v1/tasks/templates?id=xxx — delete a template
func (g *Gateway) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if g.templateStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "template store not available")
		return
	}

	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			tpl, ok := g.templateStore.Get(id)
			if !ok {
				apperror.WriteCode(w, apperror.CodeNotFound, "template not found")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tpl)
			return
		}
		all := g.templateStore.List()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"templates": all, "total": len(all)})

	case http.MethodPost:
		var tpl task.Template
		if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if err := g.templateStore.Create(&tpl); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(tpl)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		if !g.templateStore.Delete(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "template not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"deleted": id})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/DELETE only")
	}
}

// handleTemplateInstantiate creates a task from a template.
// POST /v1/tasks/templates/instantiate { "template_id": "tpl-1", "variables": {"key":"value"} }
func (g *Gateway) handleTemplateInstantiate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.templateStore == nil || g.taskStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}

	var req struct {
		TemplateID string            `json:"template_id"`
		Variables  map[string]string `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.TemplateID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "template_id is required")
		return
	}

	tenantID := tenantFromCtx(r.Context())
	t, err := g.templateStore.Instantiate(req.TemplateID, req.Variables, tenantID)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	// Save the instantiated task
	created, err := g.taskStore.Create(task.CreateRequest{
		Title:       t.Title,
		Description: t.Description,
		TenantID:    tenantID,
	})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	// Overwrite with template-defined steps (skip LLM planning)
	created.Steps = t.Steps
	g.taskStore.Update(created)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

// handleTaskThread manages task-scoped conversation threads.
// GET  /v1/tasks/threads?id=xxx            → get thread messages + info (single thread)
// GET  /v1/tasks/threads                   → list all threads (optional: ?state=open|paused|closed)
// POST /v1/tasks/threads { "task_id": "xxx", "content": "..." } → post a user message
// PUT  /v1/tasks/threads { "task_id": "xxx", "state": "paused" } → update thread state
func (g *Gateway) handleTaskThread(w http.ResponseWriter, r *http.Request) {
	if g.threadMgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task threads not available")
		return
	}

	switch r.Method {
	case http.MethodGet:
		taskID := r.URL.Query().Get("id")
		if taskID != "" {
			// Single thread query
			info := g.threadMgr.Info(taskID)
			msgs := g.threadMgr.Messages(taskID)
			if msgs == nil {
				msgs = []llm.Message{}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"task_id":  taskID,
				"info":     info,
				"messages": msgs,
			})
			return
		}
		// List all threads
		stateFilter := task.ThreadState(r.URL.Query().Get("state"))
		threads := g.threadMgr.List(stateFilter)
		if threads == nil {
			threads = []task.ThreadInfo{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"threads": threads,
			"total":   len(threads),
		})

	case http.MethodPost:
		var req struct {
			TaskID  string               `json:"task_id"`
			Content string               `json:"content"`
			Channel *task.ChannelBinding `json:"channel,omitempty"` // optional: bind to channel
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.TaskID == "" || req.Content == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "task_id and content are required")
			return
		}

		tid := tenantFromCtx(r.Context())

		// Bind channel if provided
		if req.Channel != nil {
			g.threadMgr.EnsureWithBinding(req.TaskID, tid, req.Channel)
		}

		// Check thread state — closed threads are read-only
		if state := g.threadMgr.GetState(req.TaskID); state == task.ThreadClosed {
			apperror.WriteCode(w, apperror.CodeBadRequest, "thread is closed (task completed/failed)")
			return
		}

		g.threadMgr.Post(req.TaskID, tid, "user", req.Content)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "posted",
			"task_id": req.TaskID,
		})

	case http.MethodPut:
		var req struct {
			TaskID string           `json:"task_id"`
			State  task.ThreadState `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.TaskID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "task_id is required")
			return
		}
		if req.State != task.ThreadOpen && req.State != task.ThreadPaused && req.State != task.ThreadClosed {
			apperror.WriteCode(w, apperror.CodeBadRequest, "state must be open/paused/closed")
			return
		}
		if !g.threadMgr.HasThread(req.TaskID) {
			apperror.WriteCode(w, apperror.CodeNotFound, "thread not found")
			return
		}

		g.threadMgr.SetState(req.TaskID, req.State)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "updated",
			"task_id": req.TaskID,
			"state":   string(req.State),
		})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/PUT only")
	}
}
