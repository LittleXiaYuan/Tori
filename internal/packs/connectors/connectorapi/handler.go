package connectorapi

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/connectors"
)

// Route declares one connector HTTP route.
type Route struct {
	Method      string
	Path        string
	Description string
	Handler     http.HandlerFunc
}

// Handler serves connector management HTTP endpoints.
type Handler struct {
	Registry     *connectors.Registry
	RegistryFunc func() *connectors.Registry
}

// RouteSpecs returns the connector surface without mounting it. Pack Runtime
// uses this to own route registration while preserving the existing handler
// implementation.
func (h *Handler) RouteSpecs() []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/api/connectors", Description: "List available connector definitions and connection status.", Handler: h.handleList},
		{Method: http.MethodGet, Path: "/api/connectors/detail", Description: "Read one connector definition and connection status.", Handler: h.handleDetail},
		{Method: http.MethodPost, Path: "/api/connectors/connect", Description: "Connect a connector with an API key or token.", Handler: h.handleConnect},
		{Method: http.MethodPost, Path: "/api/connectors/disconnect", Description: "Disconnect a connector and remove saved credentials.", Handler: h.handleDisconnect},
		{Method: http.MethodPost, Path: "/api/connectors/execute", Description: "Execute one action on a connected connector.", Handler: h.handleExecute},
	}
}

type connectorView struct {
	ID             string                     `json:"id"`
	Name           string                     `json:"name"`
	Description    string                     `json:"description"`
	Icon           string                     `json:"icon"`
	Category       string                     `json:"category"`
	AuthType       string                     `json:"auth_type"`
	Beta           bool                       `json:"beta,omitempty"`
	Supported      bool                       `json:"supported"`
	Status         string                     `json:"status"`
	UserInfo       string                     `json:"user_info,omitempty"`
	Error          string                     `json:"error,omitempty"`
	ActionCount    int                        `json:"action_count"`
	AllowlistCount int                        `json:"allowlist_count"`
	AllowedActions []string                   `json:"allowed_actions,omitempty"`
	LastEvent      *connectors.ConnectorEvent `json:"last_event,omitempty"`
}

func (h *Handler) registry() *connectors.Registry {
	if h.RegistryFunc != nil {
		return h.RegistryFunc()
	}
	return h.Registry
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	registry := h.registry()
	if registry == nil {
		writeJSON(w, map[string]any{"connectors": []any{}, "error": "connector system not initialized"})
		return
	}

	defs := registry.ListDefs()
	views := make([]connectorView, 0, len(defs))
	for _, d := range defs {
		inst := registry.GetInstance(d.ID)
		supported := registry.HasHandler(d.ID)
		views = append(views, connectorView{
			ID:             d.ID,
			Name:           d.Name,
			Description:    d.Description,
			Icon:           d.Icon,
			Category:       d.Category,
			AuthType:       d.AuthType,
			Beta:           d.Beta,
			Supported:      supported,
			Status:         string(inst.Status),
			UserInfo:       inst.UserInfo,
			Error:          inst.Error,
			ActionCount:    len(d.Actions),
			AllowlistCount: len(allowedConnectorActions(d, supported)),
			AllowedActions: allowedConnectorActions(d, supported),
			LastEvent:      inst.LastEvent,
		})
	}
	writeJSON(w, map[string]any{"connectors": views})
}

func (h *Handler) handleDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	registry := h.registry()
	if registry == nil {
		writeJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	connID := r.URL.Query().Get("id")
	if connID == "" {
		writeJSONStatus(w, 400, map[string]any{"error": "id parameter required"})
		return
	}
	def := registry.GetDef(connID)
	if def == nil {
		writeJSONStatus(w, 404, map[string]any{"error": "connector not found"})
		return
	}
	inst := registry.GetInstance(connID)
	supported := registry.HasHandler(connID)
	writeJSON(w, map[string]any{
		"connector":       def,
		"supported":       supported,
		"status":          string(inst.Status),
		"user_info":       inst.UserInfo,
		"error":           inst.Error,
		"allowed_actions": allowedConnectorActions(def, supported),
		"last_event":      inst.LastEvent,
	})
}

func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	registry := h.registry()
	if registry == nil {
		writeJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	var req struct {
		ConnectorID string `json:"connector_id"`
		Token       string `json:"token"`
		APIKey      string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": "invalid request body"})
		return
	}
	if req.ConnectorID == "" {
		writeJSONStatus(w, 400, map[string]any{"error": "connector_id required"})
		return
	}
	key := req.Token
	if key == "" {
		key = req.APIKey
	}
	if key == "" {
		writeJSONStatus(w, 400, map[string]any{"error": "token or api_key required"})
		return
	}
	if err := registry.ConnectWithKey(r.Context(), req.ConnectorID, key); err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	inst := registry.GetInstance(req.ConnectorID)
	writeJSON(w, map[string]any{
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
	registry := h.registry()
	if registry == nil {
		writeJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	var req struct {
		ConnectorID string `json:"connector_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": "invalid request body"})
		return
	}
	if err := registry.Disconnect(r.Context(), req.ConnectorID); err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	registry := h.registry()
	if registry == nil {
		writeJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	var req struct {
		ConnectorID string         `json:"connector_id"`
		ActionID    string         `json:"action_id"`
		Params      map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": "invalid request body"})
		return
	}
	result, err := registry.Execute(r.Context(), req.ConnectorID, req.ActionID, req.Params)
	if err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "result": result})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func allowedConnectorActions(def *connectors.ConnectorDef, supported bool) []string {
	if def == nil || !supported || len(def.Actions) == 0 {
		return nil
	}
	out := make([]string, 0, len(def.Actions))
	for _, action := range def.Actions {
		if action.ID != "" {
			out = append(out, action.ID)
		}
	}
	return out
}
