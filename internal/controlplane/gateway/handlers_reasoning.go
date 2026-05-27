package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/cognikernel"
	"yunque-agent/internal/execution/channel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
)

//	from handlers_state.go
//
// handleStateSnapshot GET /v1/state 鈥?杩斿洖瀹屾暣鐘舵€佸揩鐓?
func (g *Gateway) handleStateSnapshot(w http.ResponseWriter, r *http.Request) {
	if g.stateKernel == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return
	}
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
		return
	}
	snap := g.stateKernel.TakeSnapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

// handleStateGoals GET/POST/DELETE /v1/state/goals
func (g *Gateway) handleStateGoals(w http.ResponseWriter, r *http.Request) {
	if g.stateKernel == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		goals := g.stateKernel.Goals()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(goals)

	case http.MethodPost:
		var req struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Priority    int      `json:"priority"`
			Status      string   `json:"status"`
			Progress    float64  `json:"progress"`
			ParentGoal  string   `json:"parent_goal"`
			TaskIDs     []string `json:"task_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Title == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "title required")
			return
		}

		// If ID is provided, try to update
		if req.ID != "" {
			if g.stateKernel.UpdateGoal(req.ID, func(g *state.Goal) {
				if req.Title != "" {
					g.Title = req.Title
				}
				if req.Description != "" {
					g.Description = req.Description
				}
				if req.Priority > 0 {
					g.Priority = req.Priority
				}
				if req.Status != "" {
					g.Status = req.Status
				}
				if req.Progress > 0 {
					g.Progress = req.Progress
				}
			}) {
				g.stateKernel.Save()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"id": req.ID, "status": "updated"})
				return
			}
		}

		// Create new goal
		id := g.stateKernel.AddGoal(state.Goal{
			ID:          req.ID,
			Title:       req.Title,
			Description: req.Description,
			Priority:    req.Priority,
			ParentGoal:  req.ParentGoal,
			TaskIDs:     req.TaskIDs,
		})
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "created"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		if !g.stateKernel.RemoveGoal(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "goal not found")
			return
		}
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

// handleStateFocus GET/POST /v1/state/focus
func (g *Gateway) handleStateFocus(w http.ResponseWriter, r *http.Request) {
	if g.stateKernel == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"focus": g.stateKernel.Focus()})

	case http.MethodPost:
		var req struct {
			Focus  string   `json:"focus"`
			Topics []string `json:"topics"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Focus != "" {
			g.stateKernel.SetFocus(req.Focus)
		}
		for _, t := range req.Topics {
			g.stateKernel.AddTopic(t)
		}
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

// handleStateResources GET/POST/DELETE /v1/state/resources
func (g *Gateway) handleStateResources(w http.ResponseWriter, r *http.Request) {
	if g.stateKernel == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		res := g.stateKernel.Resources()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)

	case http.MethodPost:
		var req state.Resource
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Path == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "path required")
			return
		}
		g.stateKernel.TrackResource(req)
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "tracked"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		if !g.stateKernel.ReleaseResource(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "resource not found")
			return
		}
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "released"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

//	from handlers_react.go
//
// handleReact handles POST /v1/react to add emoji reactions to messages.
func (g *Gateway) handleReact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChannelType string `json:"channel_type"` // "telegram", "discord", etc.
		Target      string `json:"target"`       // chat ID
		MessageID   string `json:"message_id"`   // message to react to
		Emoji       string `json:"emoji"`        // unicode emoji or custom emoji ID; empty to clear
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChannelType == "" || req.Target == "" || req.MessageID == "" {
		http.Error(w, `{"error":"channel_type, target, and message_id required"}`, http.StatusBadRequest)
		return
	}

	if g.channelReg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	ch, ok := g.channelReg.Get(req.ChannelType)
	if !ok {
		http.Error(w, `{"error":"channel not found"}`, http.StatusNotFound)
		return
	}

	reactor, ok := ch.(channel.Reactor)
	if !ok {
		http.Error(w, `{"error":"channel does not support reactions"}`, http.StatusBadRequest)
		return
	}

	if err := reactor.React(r.Context(), req.Target, req.MessageID, req.Emoji); err != nil {
		slog.Error("react failed", "channel", req.ChannelType, "err", err)
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "reaction failed", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleSendSticker handles POST /v1/sticker/send to send stickers via channels.
func (g *Gateway) handleSendSticker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChannelType string `json:"channel_type"`
		Target      string `json:"target"`
		PackageID   string `json:"package_id"`
		StickerID   string `json:"sticker_id"`
		FileID      string `json:"file_id,omitempty"`
		Emoji       string `json:"emoji,omitempty"`
		Platform    string `json:"platform,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChannelType == "" || req.Target == "" {
		http.Error(w, `{"error":"channel_type and target required"}`, http.StatusBadRequest)
		return
	}

	if g.channelReg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	ch, ok := g.channelReg.Get(req.ChannelType)
	if !ok {
		http.Error(w, `{"error":"channel not found"}`, http.StatusNotFound)
		return
	}

	sender, ok := ch.(channel.StickerSender)
	if !ok {
		http.Error(w, `{"error":"channel does not support sticker sending"}`, http.StatusBadRequest)
		return
	}

	sticker := channel.NewSticker(req.PackageID, req.StickerID)
	sticker.FileID = req.FileID
	sticker.Emoji = req.Emoji
	sticker.Platform = req.Platform
	if sticker.Platform == "" {
		sticker.Platform = req.ChannelType
	}

	if err := sender.SendSticker(r.Context(), req.Target, sticker); err != nil {
		slog.Error("sendSticker failed", "channel", req.ChannelType, "err", err)
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "sticker send failed", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// PreAckReact sends a random emoji reaction to a message as acknowledgment.
// This is called before processing, so the user sees immediate feedback.
// Similar to AstrBot's preprocess_stage pre_ack_emoji.
func (g *Gateway) PreAckReact(ctx context.Context, channelType, target, messageID string) {
	if len(g.preAckEmojis) == 0 || g.channelReg == nil {
		return
	}

	ch, ok := g.channelReg.Get(channelType)
	if !ok {
		return
	}

	reactor, ok := ch.(channel.Reactor)
	if !ok {
		return
	}

	emoji := g.preAckEmojis[rand.IntN(len(g.preAckEmojis))]
	if err := reactor.React(ctx, target, messageID, emoji); err != nil {
		slog.Debug("pre-ack react failed", "channel", channelType, "err", err)
	}
}

//  from handlers_reflect.go
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
// Reflection API 鈥?experience store and strategy compilation
//
// GET /v1/reflect/experiences       鈥?list experiences (filter by source/category/outcome, ?stats=true)
// GET /v1/reflect/strategies        鈥?compiled strategy hints for LLM context
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

func (g *Gateway) handleExperiences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST only")
		return
	}
	if g.experienceStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "experience store not initialized")
		return
	}

	if r.Method == http.MethodPost {
		g.handleExperienceCreate(w, r)
		return
	}

	source := r.URL.Query().Get("source")
	category := r.URL.Query().Get("category")
	outcome := r.URL.Query().Get("outcome")
	tag := r.URL.Query().Get("tag")
	limit := reflectExperienceLimit(r, 0)

	// Stats mode supports the same filters as list/search so dashboards can ask
	// for scoped counters without fetching the full experience list.
	if r.URL.Query().Get("stats") == "true" {
		all := g.experienceStore.All()
		filtered := filterReflectExperiences(all, source, category, outcome, tag)
		if r.URL.Query().Get("kind") == "workload_feedback" {
			st := reflectpkg.SummarizeWorkloadFeedback(filtered, reflectWorkloadIDsFromQuery(r))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(st)
			return
		}
		st := summarizeReflectExperiences(filtered)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
		return
	}

	// Search mode still respects source/category/outcome filters. This keeps the
	// lightweight SDK's combined q+filter query semantics aligned with runtime.
	query := r.URL.Query().Get("q")
	if query != "" {
		results := limitReflectExperiences(filterReflectExperiences(queryReflectExperiences(g.experienceStore.All(), query), source, category, outcome, tag), limit)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"experiences": results, "total": len(results)})
		return
	}

	// List all
	all := g.experienceStore.All()
	// Apply filters
	if source != "" || category != "" || outcome != "" || tag != "" {
		filtered := limitReflectExperiences(filterReflectExperiences(all, source, category, outcome, tag), limit)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"experiences": filtered, "total": len(filtered)})
		return
	}

	all = limitReflectExperiences(all, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"experiences": all, "total": len(all)})
}

func (g *Gateway) handleExperienceCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Experience *reflectpkg.Experience `json:"experience"`
		ID         string                 `json:"id"`
		Source     string                 `json:"source"`
		SourceID   string                 `json:"source_id"`
		Category   string                 `json:"category"`
		Outcome    string                 `json:"outcome"`
		Lesson     string                 `json:"lesson"`
		Context    string                 `json:"context"`
		Tags       []string               `json:"tags"`
		CreatedAt  time.Time              `json:"created_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
		return
	}
	exp := reflectpkg.Experience{
		ID:        req.ID,
		Source:    req.Source,
		SourceID:  req.SourceID,
		Category:  req.Category,
		Outcome:   req.Outcome,
		Lesson:    req.Lesson,
		Context:   req.Context,
		Tags:      req.Tags,
		CreatedAt: req.CreatedAt,
	}
	if req.Experience != nil {
		exp = *req.Experience
	}
	exp.Source = strings.TrimSpace(exp.Source)
	exp.Category = strings.TrimSpace(exp.Category)
	exp.Outcome = strings.TrimSpace(exp.Outcome)
	exp.Lesson = strings.TrimSpace(exp.Lesson)
	exp.Context = strings.TrimSpace(exp.Context)
	exp.SourceID = strings.TrimSpace(exp.SourceID)
	if exp.Source == "" {
		exp.Source = "interaction"
	}
	if exp.Category == "" {
		exp.Category = "workload_feedback"
	}
	if exp.Outcome == "" {
		exp.Outcome = "partial"
	}
	if exp.Lesson == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "experience lesson is required")
		return
	}
	if isReflectiveLoopFeedback(exp) && g.reflectiveLoop != nil {
		if result, err := g.reflectiveLoop.IngestFeedback(r.Context(), cognikernel.FeedbackData{
			Source:    exp.Source,
			SourceID:  exp.SourceID,
			Category:  exp.Category,
			Outcome:   exp.Outcome,
			Lesson:    exp.Lesson,
			Context:   exp.Context,
			Tags:      exp.Tags,
			CreatedAt: exp.CreatedAt,
		}); err == nil && result != nil && result.ExperiencesAdded > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"experience": exp, "status": "stored", "ingested_by": "reflective_loop"})
			return
		} else if err != nil {
			slog.Warn("reflective loop feedback ingestion failed; storing directly", "err", err)
		}
	}

	g.experienceStore.Add(exp)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"experience": exp, "status": "stored"})
}

func isReflectiveLoopFeedback(exp reflectpkg.Experience) bool {
	return exp.Source == "workload_feedback" || exp.Category == "workload_feedback"
}

func reflectExperienceLimit(r *http.Request, fallback int) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return fallback
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return fallback
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func reflectWorkloadIDsFromQuery(r *http.Request) []string {
	raw := r.URL.Query().Get("workloads")
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func filterReflectExperiences(experiences []reflectpkg.Experience, source, category, outcome, tag string) []reflectpkg.Experience {
	if source == "" && category == "" && outcome == "" && tag == "" {
		return experiences
	}
	filtered := make([]reflectpkg.Experience, 0, len(experiences))
	for _, e := range experiences {
		if source != "" && e.Source != source {
			continue
		}
		if category != "" && e.Category != category {
			continue
		}
		if outcome != "" && e.Outcome != outcome {
			continue
		}
		if tag != "" && !reflectExperienceHasTag(e, tag) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func reflectExperienceHasTag(e reflectpkg.Experience, tag string) bool {
	for _, t := range e.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

func queryReflectExperiences(experiences []reflectpkg.Experience, query string) []reflectpkg.Experience {
	filtered := make([]reflectpkg.Experience, 0, len(experiences))
	for _, e := range experiences {
		if reflectpkg.MatchesQuery(e, query) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func limitReflectExperiences(experiences []reflectpkg.Experience, limit int) []reflectpkg.Experience {
	if limit <= 0 || len(experiences) <= limit {
		return experiences
	}
	return experiences[:limit]
}

func summarizeReflectExperiences(experiences []reflectpkg.Experience) reflectpkg.ExperienceStats {
	st := reflectpkg.ExperienceStats{
		Total:      len(experiences),
		BySource:   make(map[string]int),
		ByCategory: make(map[string]int),
		ByOutcome:  make(map[string]int),
	}
	week := time.Now().Add(-7 * 24 * time.Hour)
	for _, e := range experiences {
		st.BySource[e.Source]++
		st.ByCategory[e.Category]++
		st.ByOutcome[e.Outcome]++
		if e.CreatedAt.After(week) {
			st.Recent++
		}
	}
	return st
}

func (g *Gateway) handleStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.experienceStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "experience store not initialized")
		return
	}

	limit := reflectExperienceLimit(r, 20)
	source := r.URL.Query().Get("source")
	category := r.URL.Query().Get("category")
	outcome := r.URL.Query().Get("outcome")
	tag := r.URL.Query().Get("tag")
	query := r.URL.Query().Get("q")

	strategies := ""
	if source != "" || category != "" || outcome != "" || tag != "" || query != "" {
		experiences := g.experienceStore.All()
		if query != "" {
			experiences = queryReflectExperiences(experiences, query)
		}
		strategies = reflectpkg.CompileStrategiesFrom(filterReflectExperiences(experiences, source, category, outcome, tag), limit)
	} else {
		strategies = g.experienceStore.CompileStrategies(limit)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"strategies": strategies})
}
