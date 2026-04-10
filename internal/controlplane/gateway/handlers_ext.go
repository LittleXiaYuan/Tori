package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/pkg/safego"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/apperror"
)

// --- Persona API ---

func (g *Gateway) handlePersona(w http.ResponseWriter, r *http.Request) {
	if g.persona == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "persona not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"identity": g.persona.Identity(),
			"soul":     g.persona.Soul(),
			"skills":   g.persona.Skills(),
		})
	case http.MethodPut:
		var req struct {
			Identity *string `json:"identity"`
			Soul     *string `json:"soul"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid json")
			return
		}
		if req.Identity != nil {
			if err := g.persona.SetIdentity(*req.Identity); err != nil {
				apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "set identity", err))
				return
			}
		}
		if req.Soul != nil {
			if err := g.persona.SetSoul(*req.Soul); err != nil {
				apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "set soul", err))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT")
	}
}

func (g *Gateway) handlePersonaSkills(w http.ResponseWriter, r *http.Request) {
	if g.persona == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "persona not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		skills := g.persona.Skills()
		if skills == nil {
			skills = []persona.Skill{}
		}
		json.NewEncoder(w).Encode(map[string]any{"skills": skills})
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Content     string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		if err := g.persona.AddSkill(req.Name, req.Description, req.Content); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "add skill", err))
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodDelete:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		g.persona.DeleteSkill(req.Name)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET, POST, or DELETE")
	}
}

// --- Conversation API ---

func (g *Gateway) handleConversations(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	w.Header().Set("Content-Type", "application/json")
	sessions := g.convStore.ListByTenant(tid)

	// Filter: exclude archived unless ?archived=true
	showArchived := r.URL.Query().Get("archived") == "true"
	var filtered []session.Session
	for _, s := range sessions {
		if s.ArchivedAt != nil && !showArchived {
			continue
		}
		filtered = append(filtered, s)
	}
	json.NewEncoder(w).Encode(map[string]any{"sessions": filtered, "count": len(filtered)})
}

func (g *Gateway) handleConversationMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "session_id query param required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		msgs := g.convStore.Get(sessionID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"messages": msgs, "count": len(msgs)})
	case http.MethodDelete:
		g.convStore.Delete(sessionID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or DELETE only")
	}
}

// handleConversationManage handles rename, pin, archive operations on a conversation.
func (g *Gateway) handleConversationManage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "PUT only")
		return
	}
	var req struct {
		SessionID string  `json:"session_id"`
		Name      *string `json:"name,omitempty"`
		Pinned    *bool   `json:"pinned,omitempty"`
		Archive   *bool   `json:"archive,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid body")
		return
	}
	if req.SessionID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "session_id required")
		return
	}

	if req.Name != nil {
		g.convStore.Rename(req.SessionID, *req.Name)
	}
	if req.Pinned != nil {
		g.convStore.Pin(req.SessionID, *req.Pinned)
	}
	if req.Archive != nil {
		if *req.Archive {
			g.convStore.Archive(req.SessionID)
		} else {
			g.convStore.Unarchive(req.SessionID)
		}
	}

	sess := g.convStore.GetSession(req.SessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "updated", "session": sess})
}

// --- Feishu Webhook ---

func (g *Gateway) handleFeishuWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var body struct {
		Challenge string `json:"challenge"` // URL verification
		Type      string `json:"type"`
		Event     struct {
			Message struct {
				ChatID  string `json:"chat_id"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"event"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	// Feishu URL verification
	if body.Challenge != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": body.Challenge})
		return
	}
	// Process message async and reply via Feishu API
	if body.Event.Message.Content != "" {
		safego.Go("feishu-webhook-reply", func() {
			ctx := context.Background()
			result, err := g.planner.Run(ctx, planner.PlanRequest{
				Messages: []llm.Message{{Role: "user", Content: body.Event.Message.Content}},
				TenantID: "default",
			})
			if err != nil {
				slog.Error("feishu webhook planner error", "err", err)
				return
			}
			// Send reply back to Feishu chat
			if g.feishuAPI != nil && body.Event.Message.ChatID != "" {
				if err := g.feishuAPI.SendMessage(body.Event.Message.ChatID, result.Reply); err != nil {
					slog.Error("feishu reply error", "err", err)
				}
			}
			slog.Info("feishu webhook reply", "chat_id", body.Event.Message.ChatID, "len", len(result.Reply))
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- Smart Router API ---

func (g *Gateway) handleRouterStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.smartRouter == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"slots": g.smartRouter.GetSlots(),
		"stats": g.smartRouter.GetStats(),
	})
}

// --- Identity API ---

func (g *Gateway) handleIdentityResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.identityRes == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "identity resolver not configured"})
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	var req struct {
		Channel     string `json:"channel"`
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
		return
	}
	profile := g.identityRes.Resolve(req.Channel, req.UserID, req.DisplayName)
	json.NewEncoder(w).Encode(profile)
}

func (g *Gateway) handleIdentityProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.identityRes == nil {
		json.NewEncoder(w).Encode(map[string]any{"profiles": []any{}})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"profiles": g.identityRes.All()})
}

// --- Cost Tracking API ---

func (g *Gateway) handleCostSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "not configured"})
		return
	}
	summary := g.costTracker.GetSummary()
	today := g.costTracker.TodayCost()
	json.NewEncoder(w).Encode(map[string]any{
		"summary":    summary,
		"today_cost": today,
		"month_cost": g.costTracker.MonthCost(),
	})
}

func (g *Gateway) handleCostBudget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	var budget costtrack.Budget
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid budget")
		return
	}
	g.costTracker.SetBudget(budget)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// handleCostByTask returns cost breakdown for a specific task.
// GET /v1/cost/task?id=xxx
func (g *Gateway) handleCostByTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	json.NewEncoder(w).Encode(g.costTracker.GetTaskCost(taskID))
}

// handleCostBreakdown returns cost breakdowns by channel, tier, runner type, and provider.
// GET /v1/cost/breakdown
func (g *Gateway) handleCostBreakdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"by_channel":     g.costTracker.GetCostByChannel(),
		"by_tier":        g.costTracker.GetCostByTier(),
		"by_runner_type": g.costTracker.GetCostByRunnerType(),
		"by_provider":    g.costTracker.GetCostByProvider(),
	})
}

// handleCostTaskTimeline returns the ordered usage events for a single task.
// GET /v1/cost/task/timeline?id=xxx
func (g *Gateway) handleCostTaskTimeline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	json.NewEncoder(w).Encode(g.costTracker.GetTaskTimeline(taskID))
}

// handleCostHistory returns paginated usage records with optional filters.
// GET /v1/cost/history?page=1&limit=50&task_id=&model=&channel=&runner_type=&provider_id=
func (g *Gateway) handleCostHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	q := r.URL.Query()
	page := 1
	if v := q.Get("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	limit := 50
	if v := q.Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	if limit > 200 {
		limit = 200
	}
	f := costtrack.UsageFilter{
		TaskID:     q.Get("task_id"),
		Model:      q.Get("model"),
		Channel:    q.Get("channel"),
		RunnerType: q.Get("runner_type"),
		ProviderID: q.Get("provider_id"),
		Page:       page,
		Limit:      limit,
	}
	json.NewEncoder(w).Encode(g.costTracker.GetUsageHistory(f))
}

// handleCostAlerts returns alert history.
// GET /v1/cost/alerts
func (g *Gateway) handleCostAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"alerts":     g.costTracker.GetAlerts(),
		"today_cost": g.costTracker.TodayCost(),
		"month_cost": g.costTracker.MonthCost(),
	})
}

// --- Fork Tree API ---

// persistForkTree saves the fork tree to disk if a persister is configured.
func (g *Gateway) persistForkTree() {
	if g.forkPersister != nil && g.forkTree != nil {
		safego.Go("fork-tree-persist", func() {
			if err := g.forkPersister.Save(g.forkTree); err != nil {
				slog.Error("fork tree persist failed", "err", err)
			}
		})
	}
}

func (g *Gateway) handleFork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.forkTree == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "fork tree not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		forkID := r.URL.Query().Get("id")
		if forkID != "" {
			f, ok := g.forkTree.Get(forkID)
			if !ok {
				apperror.WriteCode(w, apperror.CodeBadRequest, "fork not found")
				return
			}
			json.NewEncoder(w).Encode(f)
		} else {
			sessionID := r.URL.Query().Get("session_id")
			if sessionID == "" {
				apperror.WriteCode(w, apperror.CodeBadRequest, "session_id or id required")
				return
			}
			root, ok := g.forkTree.GetRoot(sessionID)
			if !ok {
				json.NewEncoder(w).Encode(map[string]any{"fork": nil})
				return
			}
			json.NewEncoder(w).Encode(root)
		}
	case http.MethodPost:
		var req struct {
			SessionID string                `json:"session_id"`
			Messages  []session.ForkMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
			return
		}
		f := g.forkTree.Create(req.SessionID, req.Messages)
		g.persistForkTree()
		json.NewEncoder(w).Encode(f)
	case http.MethodDelete:
		forkID := r.URL.Query().Get("id")
		if forkID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		ok := g.forkTree.Delete(forkID)
		g.persistForkTree()
		json.NewEncoder(w).Encode(map[string]bool{"deleted": ok})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (g *Gateway) handleForkBranch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.forkTree == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "fork tree not configured"})
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	var req struct {
		ForkID  string `json:"fork_id"`
		AtIndex int    `json:"at_index"`
		Label   string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
		return
	}
	branch, err := g.forkTree.Branch(req.ForkID, req.AtIndex, req.Label)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	g.persistForkTree()
	json.NewEncoder(w).Encode(branch)
}

func (g *Gateway) handleForkList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.forkTree == nil {
		json.NewEncoder(w).Encode(map[string]any{"forks": []any{}})
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "session_id required")
		return
	}
	forks := g.forkTree.ListBranches(sessionID)
	json.NewEncoder(w).Encode(map[string]any{"forks": forks})
}

// --- Embeddings API ---

func (g *Gateway) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.embedResolver == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "embeddings not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(map[string]any{
			"providers": g.embedResolver.List(),
		})
	case http.MethodPost:
		var req struct {
			Text     string `json:"text"`
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
			return
		}
		embedder, ok := g.embedResolver.Primary()
		if req.Provider != "" {
			embedder, ok = g.embedResolver.Get(req.Provider)
		}
		if !ok {
			apperror.WriteCode(w, apperror.CodeBadRequest, "no embedder available")
			return
		}
		vec, err := embedder.Embed(r.Context(), req.Text)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeLLMError, "embedding failed: "+err.Error())
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"embedding":  vec,
			"dimensions": len(vec),
			"model":      embedder.Model(),
		})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

// --- Subagent API ---

func (g *Gateway) handleSubagent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.subagentMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "subagent manager not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			sa, ok := g.subagentMgr.Get(id)
			if !ok {
				apperror.WriteCode(w, apperror.CodeBadRequest, "subagent not found")
				return
			}
			json.NewEncoder(w).Encode(sa)
		} else {
			parentID := r.URL.Query().Get("parent_id")
			json.NewEncoder(w).Encode(map[string]any{"subagents": g.subagentMgr.List(parentID)})
		}
	case http.MethodPost:
		var req struct {
			ParentID    string   `json:"parent_id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Skills      []string `json:"skills"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
			return
		}
		sa, err := g.subagentMgr.Spawn(req.ParentID, req.Name, req.Description, req.Skills)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		json.NewEncoder(w).Encode(sa)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		ok := g.subagentMgr.Destroy(id)
		json.NewEncoder(w).Encode(map[string]bool{"deleted": ok})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (g *Gateway) handleSubagentMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.subagentMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "subagent manager not configured"})
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	var req struct {
		ID       string           `json:"id"`
		Messages []map[string]any `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
		return
	}
	if err := g.subagentMgr.AppendMessages(req.ID, req.Messages); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// --- Plugin UI Discovery API ---

// handlePluginUI returns all UI tabs from enabled plugins that implement UIPlugin.
// GET /v1/plugins/ui
func (g *Gateway) handlePluginUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET required")
		return
	}

	tabs := g.pluginReg.AllUITabs()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tabs": tabs,
	})
}
