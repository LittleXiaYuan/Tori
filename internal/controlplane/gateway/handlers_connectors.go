package gateway

import (
	"encoding/json"
	"net/http"
)

// GET /api/connectors — list all connectors with their status
func (g *Gateway) handleConnectorList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}

	reg := g.connectorReg
	if reg == nil {
		writeJSON(w, map[string]any{"connectors": []any{}, "error": "connector system not initialized"})
		return
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

	defs := reg.ListDefs()
	views := make([]connectorView, 0, len(defs))
	for _, d := range defs {
		inst := reg.GetInstance(d.ID)
		views = append(views, connectorView{
			ID:          d.ID,
			Name:        d.Name,
			Description: d.Description,
			Icon:        d.Icon,
			Category:    d.Category,
			AuthType:    d.AuthType,
			Beta:        d.Beta,
			Supported:   reg.HasHandler(d.ID),
			Status:      string(inst.Status),
			UserInfo:    inst.UserInfo,
			Error:       inst.Error,
			ActionCount: len(d.Actions),
		})
	}

	writeJSON(w, map[string]any{"connectors": views})
}

// GET /api/connectors/detail?id=xxx — get connector details including actions
func (g *Gateway) handleConnectorDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}

	reg := g.connectorReg
	if reg == nil {
		writeJSONStatus(w, 500, map[string]any{"error": "not initialized"})
		return
	}

	connID := r.URL.Query().Get("id")
	if connID == "" {
		writeJSONStatus(w, 400, map[string]any{"error": "id parameter required"})
		return
	}

	def := reg.GetDef(connID)
	if def == nil {
		writeJSONStatus(w, 404, map[string]any{"error": "connector not found"})
		return
	}

	inst := reg.GetInstance(connID)
	writeJSON(w, map[string]any{
		"connector": def,
		"supported": reg.HasHandler(connID),
		"status":    string(inst.Status),
		"user_info": inst.UserInfo,
		"error":     inst.Error,
	})
}

// POST /api/connectors/connect — connect a connector
func (g *Gateway) handleConnectorConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	reg := g.connectorReg
	if reg == nil {
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

	if err := reg.ConnectWithKey(r.Context(), req.ConnectorID, key); err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}

	inst := reg.GetInstance(req.ConnectorID)
	writeJSON(w, map[string]any{
		"ok":        true,
		"status":    string(inst.Status),
		"user_info": inst.UserInfo,
	})
}

// POST /api/connectors/disconnect — disconnect a connector
func (g *Gateway) handleConnectorDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	reg := g.connectorReg
	if reg == nil {
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

	if err := reg.Disconnect(r.Context(), req.ConnectorID); err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, map[string]any{"ok": true})
}

// POST /api/connectors/execute — execute a connector action
func (g *Gateway) handleConnectorExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	reg := g.connectorReg
	if reg == nil {
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

	result, err := reg.Execute(r.Context(), req.ConnectorID, req.ActionID, req.Params)
	if err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, map[string]any{"ok": true, "result": result})
}
