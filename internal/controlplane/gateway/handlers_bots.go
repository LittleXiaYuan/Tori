package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/apperror"
)

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
