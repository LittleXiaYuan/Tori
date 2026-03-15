package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/tools"
)

func (g *Gateway) handleToolExec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.toolsMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	var opts tools.ExecOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}
	result, err := g.toolsMgr.Exec(r.Context(), opts)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) handleToolList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.toolsMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"sessions": g.toolsMgr.List()})
}

func (g *Gateway) handleToolPoll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.toolsMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "session id required"})
		return
	}
	lines, state, err := g.toolsMgr.PollSession(id)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"lines": lines, "state": state})
}

func (g *Gateway) handleToolKill(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.toolsMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "session id required"})
		return
	}
	if err := g.toolsMgr.Kill(id); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"killed": id})
}
