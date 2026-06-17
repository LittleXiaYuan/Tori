package controlplanepack

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/apperror"
)

func (h *Handler) handleBots(w http.ResponseWriter, r *http.Request) {
	manager := h.gateway.BotManager()
	if manager == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "bot manager not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		list := manager.List()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"bots":   list,
			"total":  manager.Count(),
			"active": manager.ActiveCount(),
		})
	case http.MethodPost:
		var req struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Config      bots.BotConfig `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		bot, err := manager.Create(req.Name, req.Description, req.Config)
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeBadRequest, "create bot", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(bot)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
	}
}

func (h *Handler) handleBotDetail(w http.ResponseWriter, r *http.Request) {
	manager := h.gateway.BotManager()
	if manager == nil {
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
		bot, ok := manager.Get(id)
		if !ok {
			apperror.WriteCode(w, apperror.CodeNotFound, "bot not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(bot)
	case http.MethodPut:
		var req struct {
			Name        *string         `json:"name"`
			Description *string         `json:"description"`
			Config      *bots.BotConfig `json:"config"`
			Active      *bool           `json:"active"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Active != nil {
			if err := manager.SetActive(id, *req.Active); err != nil {
				apperror.Write(w, apperror.Wrap(apperror.CodeNotFound, "set active", err))
				return
			}
		}
		bot, err := manager.Update(id, req.Name, req.Description, req.Config)
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeNotFound, "update bot", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(bot)
	case http.MethodDelete:
		if err := manager.Delete(id); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeNotFound, "delete bot", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET, PUT, or DELETE")
	}
}
