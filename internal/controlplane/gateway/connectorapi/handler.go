package connectorapi

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/gateway/gwshared"
)

// Handler serves connector management HTTP endpoints.
type Handler struct {
	Registry *connectors.Registry
}

// RegisterRoutes mounts all /api/connectors/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	mux.HandleFunc("/api/connectors", auth(h.handleList))
	mux.HandleFunc("/api/connectors/detail", auth(h.handleDetail))
	mux.HandleFunc("/api/connectors/connect", auth(h.handleConnect))
	mux.HandleFunc("/api/connectors/disconnect", auth(h.handleDisconnect))
	mux.HandleFunc("/api/connectors/execute", auth(h.handleExecute))
}

type connectorView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	AuthType    string `json:"auth_type"`
	Beta        bool   `json:"beta,omitempty"`
	Supported   bool   `json:"supported"`
	Status      string `json:"status"`
	UserInfo    string `json:"user_info,omitempty"`
	Error       string `json:"error,omitempty"`
	ActionCount int    `json:"action_count"`
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	if h.Registry == nil {
		gwshared.WriteJSON(w, map[string]any{"connectors": []any{}, "error": "connector system not initialized"})
		return
	}

	defs := h.Registry.ListDefs()
	views := make([]connectorView, 0, len(defs))
	for _, d := range defs {
		inst := h.Registry.GetInstance(d.ID)
		views = append(views, connectorView{
			ID:          d.ID,
			Name:        d.Name,
			Description: d.Description,
			Icon:        d.Icon,
			Category:    d.Category,
			AuthType:    d.AuthType,
			Beta:        d.Beta,
			Supported:   h.Registry.HasHandler(d.ID),
			Status:      string(inst.Status),
			UserInfo:    inst.UserInfo,
			Error:       inst.Error,
			ActionCount: len(d.Actions),
		})
	}
	gwshared.WriteJSON(w, map[string]any{"connectors": views})
}

func (h *Handler) handleDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	if h.Registry == nil {
		gwshared.WriteJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	connID := r.URL.Query().Get("id")
	if connID == "" {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": "id parameter required"})
		return
	}
	def := h.Registry.GetDef(connID)
	if def == nil {
		gwshared.WriteJSONStatus(w, 404, map[string]any{"error": "connector not found"})
		return
	}
	inst := h.Registry.GetInstance(connID)
	gwshared.WriteJSON(w, map[string]any{
		"connector": def,
		"supported": h.Registry.HasHandler(connID),
		"status":    string(inst.Status),
		"user_info": inst.UserInfo,
		"error":     inst.Error,
	})
}

func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if h.Registry == nil {
		gwshared.WriteJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	var req struct {
		ConnectorID string `json:"connector_id"`
		Token       string `json:"token"`
		APIKey      string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": "invalid request body"})
		return
	}
	if req.ConnectorID == "" {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": "connector_id required"})
		return
	}
	key := req.Token
	if key == "" {
		key = req.APIKey
	}
	if key == "" {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": "token or api_key required"})
		return
	}
	if err := h.Registry.ConnectWithKey(r.Context(), req.ConnectorID, key); err != nil {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	inst := h.Registry.GetInstance(req.ConnectorID)
	gwshared.WriteJSON(w, map[string]any{
		"ok":        true,
		"status":    string(inst.Status),
		"user_info": inst.UserInfo,
	})
}

func (h *Handler) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if h.Registry == nil {
		gwshared.WriteJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	var req struct {
		ConnectorID string `json:"connector_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": "invalid request body"})
		return
	}
	if err := h.Registry.Disconnect(r.Context(), req.ConnectorID); err != nil {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if h.Registry == nil {
		gwshared.WriteJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	var req struct {
		ConnectorID string         `json:"connector_id"`
		ActionID    string         `json:"action_id"`
		Params      map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": "invalid request body"})
		return
	}
	result, err := h.Registry.Execute(r.Context(), req.ConnectorID, req.ActionID, req.Params)
	if err != nil {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	gwshared.WriteJSON(w, map[string]any{"ok": true, "result": result})
}
