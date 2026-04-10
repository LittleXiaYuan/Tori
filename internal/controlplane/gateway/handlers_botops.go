package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/channel"
)

//  from handlers_bots.go 
func (g *Gateway) handleBots(w http.ResponseWriter, r *http.Request) {
	if g.botMgr == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "bot manager not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		list := g.botMgr.List()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"bots":  list,
			"total": g.botMgr.Count(),
			"active": g.botMgr.ActiveCount(),
		})
	case http.MethodPost:
		var req struct {
			Name        string     `json:"name"`
			Description string     `json:"description"`
			Config      bots.BotConfig `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		bot, err := g.botMgr.Create(req.Name, req.Description, req.Config)
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeBadRequest, "create bot", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(bot)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
	}
}

func (g *Gateway) handleBotDetail(w http.ResponseWriter, r *http.Request) {
	if g.botMgr == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "bot manager not configured")
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		bot, ok := g.botMgr.Get(id)
		if !ok {
			apperror.WriteCode(w, apperror.CodeNotFound, "bot not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bot)
	case http.MethodPut:
		var req struct {
			Name        *string        `json:"name"`
			Description *string        `json:"description"`
			Config      *bots.BotConfig `json:"config"`
			Active      *bool          `json:"active"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Active != nil {
			if err := g.botMgr.SetActive(id, *req.Active); err != nil {
				apperror.Write(w, apperror.Wrap(apperror.CodeNotFound, "set active", err))
				return
			}
		}
		bot, err := g.botMgr.Update(id, req.Name, req.Description, req.Config)
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeNotFound, "update bot", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bot)
	case http.MethodDelete:
		if err := g.botMgr.Delete(id); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeNotFound, "delete bot", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET, PUT, or DELETE")
	}
}

//  from handlers_inbox.go 
func (g *Gateway) handleInbox(w http.ResponseWriter, r *http.Request) {
	if g.inbox == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "inbox not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		unreadOnly := r.URL.Query().Get("unread") == "true"
		items := g.inbox.List(unreadOnly, 50)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items": items,
			"count": g.inbox.Count(),
		})
	case http.MethodPost:
		var req struct {
			Source  string         `json:"source"`
			Content string         `json:"content"`
			Action  string         `json:"action"`
			Header  map[string]any `json:"header"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "content is required")
			return
		}
		item, err := g.inbox.Push(req.Source, req.Content, req.Action, req.Header)
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "push failed", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(item)
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		g.inbox.Delete(req.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET, POST, or DELETE")
	}
}

func (g *Gateway) handleInboxRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.inbox == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "inbox not configured")
		return
	}
	var req struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	count := 0
	if req.All {
		count = g.inbox.MarkAllRead()
	} else if len(req.IDs) > 0 {
		count = g.inbox.MarkRead(req.IDs)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"marked": count})
}

//  from handlers_channel_groups.go 
// handleChannelGroups handles GET /v1/channels/groups?type=telegram
// Returns all groups the bot is currently in, optionally filtered by channel type.
func (g *Gateway) handleChannelGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if g.channelReg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	typ := r.URL.Query().Get("type") // optional filter

	groups, err := g.channelReg.ListGroups(r.Context(), typ)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "list groups failed", err))
		return
	}
	if groups == nil {
		groups = make([]channel.GroupInfo, 0) // ensure JSON array, not null
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"groups": groups,
		"count":  len(groups),
	})
}

