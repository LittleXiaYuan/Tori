package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/cogni"
)

func (g *Gateway) cogniFederationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniFederation == nil {
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}
	stats := g.cogniFederation.Stats()
	stats["enabled"] = true
	json.NewEncoder(w).Encode(stats)
}

func (g *Gateway) cogniFederationPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		if g.cogniFederation == nil {
			json.NewEncoder(w).Encode(map[string]any{"peers": []any{}})
			return
		}
		peers := g.cogniFederation.Peers()
		json.NewEncoder(w).Encode(map[string]any{"peers": peers, "count": len(peers)})
	case http.MethodPost:
		if g.cogniFederation == nil {
			apperror.WriteCode(w, apperror.CodeInternal, "federation not configured")
			return
		}
		var peer cogni.FederationPeer
		if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		g.cogniFederation.AddPeer(peer)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "id": peer.ID})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
	}
}

func (g *Gateway) cogniFederationDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniFederation == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation not configured")
		return
	}
	skills := g.cogniFederation.DiscoverRemoteSkills(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"skills": skills,
		"count":  len(skills),
	})
}

func (g *Gateway) cogniFederationExpose(w http.ResponseWriter, r *http.Request, id string, expose bool) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniFederation == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation not configured")
		return
	}
	if expose {
		if err := g.cogniFederation.Expose(id); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
	} else {
		g.cogniFederation.Unexpose(id)
	}
	action := "unexposed"
	if expose {
		action = "exposed"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": action, "id": id})
}

// cogniRoute serves POST /v1/cognis/route: broadcast a message to the
// CogniBus and return the ranked bid results. This is the production surface
// of the bidding router — operators and the UI use it to inspect (and dry-run)
// "which cogni would win this turn and why", including custom Bidder output.
func (g *Gateway) cogniRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniBus == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "cogni bus not configured")
		return
	}

	var body struct {
		Message  string   `json:"message"`
		TenantID string   `json:"tenant_id"`
		Channel  string   `json:"channel"`
		Tags     []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	if body.Message == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "message is required")
		return
	}

	result := g.cogniBus.Route(r.Context(), cogni.Session{
		Message:  body.Message,
		TenantID: body.TenantID,
		Channel:  body.Channel,
		Tags:     body.Tags,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) cogniEconomics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniCostTracker == nil {
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"summary": g.cogniCostTracker.DailySummary(),
	})
}
