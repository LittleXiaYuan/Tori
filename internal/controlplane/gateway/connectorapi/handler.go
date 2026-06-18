package connectorapi

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/gateway/gwshared"
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

// RegisterRoutes mounts all /api/connectors/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	for _, route := range h.RouteSpecs() {
		mux.HandleFunc(route.Path, auth(route.Handler))
	}
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
		gwshared.WriteJSON(w, map[string]any{"connectors": []any{}, "error": "connector system not initialized"})
		return
	}

	defs := registry.ListDefs()
	views := make([]connectorView, 0, len(defs))
	for _, d := range defs {
		inst := registry.GetInstance(d.ID)
		views = append(views, connectorView{
			ID:          d.ID,
			Name:        d.Name,
			Description: d.Description,
			Icon:        d.Icon,
			Category:    d.Category,
			AuthType:    d.AuthType,
			Beta:        d.Beta,
			Supported:   registry.HasHandler(d.ID),
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
	registry := h.registry()
	if registry == nil {
		gwshared.WriteJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}
	connID := r.URL.Query().Get("id")
	if connID == "" {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": "id parameter required"})
		return
	}
	def := registry.GetDef(connID)
	if def == nil {
		gwshared.WriteJSONStatus(w, 404, map[string]any{"error": "connector not found"})
		return
	}
	inst := registry.GetInstance(connID)
	gwshared.WriteJSON(w, map[string]any{
		"connector": def,
		"supported": registry.HasHandler(connID),
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
	registry := h.registry()
	if registry == nil {
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
	if err := registry.ConnectWithKey(r.Context(), req.ConnectorID, key); err != nil {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	inst := registry.GetInstance(req.ConnectorID)
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
	registry := h.registry()
	if registry == nil {
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
	if err := registry.Disconnect(r.Context(), req.ConnectorID); err != nil {
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
	registry := h.registry()
	if registry == nil {
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
	result, err := registry.Execute(r.Context(), req.ConnectorID, req.ActionID, req.Params)
	if err != nil {
		gwshared.WriteJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	gwshared.WriteJSON(w, map[string]any{"ok": true, "result": result})
}
