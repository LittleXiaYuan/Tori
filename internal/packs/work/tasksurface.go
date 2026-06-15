package workpack

// tasksurface.go holds the de-shelled task-surface handlers (gaps, working
// memory, templates, threads) that moved out of the gateway into this pack.
// Each reaches its subsystem through a narrow WorkGateway accessor.

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/apperror"
)

// handleGaps returns capability gap records and stats.
func (h *Handler) handleGaps(w http.ResponseWriter, r *http.Request) {
	ga := h.gateway.GapAnalyzer()
	if ga == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "gap analyzer not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Query().Get("stats") == "true" {
		_ = json.NewEncoder(w).Encode(ga.Stats())
		return
	}
	gapType := task.GapType(r.URL.Query().Get("type"))
	records := ga.Records(gapType, true)
	if records == nil {
		records = []task.GapRecord{}
	}
	_ = json.NewEncoder(w).Encode(records)
}

// handleGapResolve marks a gap as resolved.
func (h *Handler) handleGapResolve(w http.ResponseWriter, r *http.Request) {
	ga := h.gateway.GapAnalyzer()
	if ga == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "gap analyzer not available")
		return
	}
	id, ok := h.decodeID(w, r)
	if !ok {
		return
	}
	if !ga.Resolve(id) {
		apperror.WriteCode(w, apperror.CodeNotFound, "gap not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"resolved": id})
}

// handleTaskWorkingMemory returns the working memory for a task.
func (h *Handler) handleTaskWorkingMemory(w http.ResponseWriter, r *http.Request) {
	mgr := h.gateway.WorkMemManager()
	if mgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "working memory not available")
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	wm := mgr.Get(taskID)
	if wm == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "no working memory for this task")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(wm)
}

// handleTemplates handles CRUD for task templates.
func (h *Handler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	store := h.gateway.TemplateStore()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "template store not available")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if id := r.URL.Query().Get("id"); id != "" {
			tpl, ok := store.Get(id)
			if !ok {
				apperror.WriteCode(w, apperror.CodeNotFound, "template not found")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tpl)
			return
		}
		all := store.List()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"templates": all, "total": len(all)})

	case http.MethodPost:
		var tpl task.Template
		if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if err := store.Create(&tpl); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(tpl)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		if !store.Delete(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "template not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"deleted": id})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/DELETE only")
	}
}

// handleTemplateInstantiate creates a task from a template.
func (h *Handler) handleTemplateInstantiate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	store := h.gateway.TemplateStore()
	taskStore := h.gateway.TaskStore()
	if store == nil || taskStore == nil {
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
	tenantID := h.gateway.TenantOf(r.Context())
	t, err := store.Instantiate(req.TemplateID, req.Variables, tenantID)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	created, err := taskStore.Create(task.CreateRequest{
		Title:       t.Title,
		Description: t.Description,
		TenantID:    tenantID,
	})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	created.Steps = t.Steps
	taskStore.Update(created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// handleTaskThread manages task-scoped conversation threads.
func (h *Handler) handleTaskThread(w http.ResponseWriter, r *http.Request) {
	tm := h.gateway.ThreadManager()
	if tm == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task threads not available")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if taskID := r.URL.Query().Get("id"); taskID != "" {
			info := tm.Info(taskID)
			msgs := tm.Messages(taskID)
			if msgs == nil {
				msgs = []llm.Message{}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"task_id": taskID, "info": info, "messages": msgs})
			return
		}
		stateFilter := task.ThreadState(r.URL.Query().Get("state"))
		threads := tm.List(stateFilter)
		if threads == nil {
			threads = []task.ThreadInfo{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"threads": threads, "total": len(threads)})

	case http.MethodPost:
		var req struct {
			TaskID  string               `json:"task_id"`
			Content string               `json:"content"`
			Channel *task.ChannelBinding `json:"channel,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.TaskID == "" || req.Content == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "task_id and content are required")
			return
		}
		tid := h.gateway.TenantOf(r.Context())
		if req.Channel != nil {
			tm.EnsureWithBinding(req.TaskID, tid, req.Channel)
		}
		if state := tm.GetState(req.TaskID); state == task.ThreadClosed {
			apperror.WriteCode(w, apperror.CodeBadRequest, "thread is closed (task completed/failed)")
			return
		}
		tm.Post(req.TaskID, tid, "user", req.Content)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "posted", "task_id": req.TaskID})

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
		if !tm.HasThread(req.TaskID) {
			apperror.WriteCode(w, apperror.CodeNotFound, "thread not found")
			return
		}
		tm.SetState(req.TaskID, req.State)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated", "task_id": req.TaskID, "state": string(req.State)})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/PUT only")
	}
}
