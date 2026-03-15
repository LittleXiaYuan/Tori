package gateway

import (
	"encoding/json"
	"net/http"
)

func (g *Gateway) handleFedPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.fedHub == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "federation not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"local_id": string(g.fedHub.LocalID()),
		"peers":    g.fedHub.ListPeers(),
	})
}

func (g *Gateway) handleFedStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.fedHub == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "federation not configured"})
		return
	}
	json.NewEncoder(w).Encode(g.fedHub.Stats())
}
