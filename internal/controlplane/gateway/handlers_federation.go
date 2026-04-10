package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/opp"
)

// handleFedCapabilities returns this agent's OPP capabilities (GET)
// or updates them (POST).
func (g *Gateway) handleFedCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if g.fedBridge == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation bridge not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		caps := g.fedBridge.LocalCaps()
		json.NewEncoder(w).Encode(map[string]any{
			"local": caps,
			"peers": g.fedBridge.ListPeerCaps(),
		})

	case http.MethodPost:
		var caps opp.CapabilitiesPayload
		if err := json.NewDecoder(r.Body).Decode(&caps); err != nil {
			apperror.WriteCode(w, apperror.CodeMissingField, "invalid capabilities payload")
			return
		}
		if err := caps.Validate(); err != nil {
			apperror.WriteCode(w, apperror.CodeMissingField, err.Error())
			return
		}
		g.fedBridge.UpdateLocalCaps(caps)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
	}
}

// handleFedDiscover searches for agents matching given requirements.
func (g *Gateway) handleFedDiscover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if g.fedBridge == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation bridge not configured")
		return
	}

	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var req struct {
		Feature  string `json:"feature"`
		Adapter  string `json:"adapter"`
		Intent   string `json:"intent"`
		MinTier  string `json:"min_tier"`
		Features []string `json:"features"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeMissingField, "invalid request")
		return
	}

	type result struct {
		PeerID   string                    `json:"peer_id"`
		AgentID  string                    `json:"agent_id"`
		Models   []opp.ModelInfo           `json:"models"`
		Adapters []opp.AdapterInfo         `json:"adapters"`
		Intents  []string                  `json:"intents"`
		Healthy  bool                      `json:"healthy"`
		Latency  int64                     `json:"latency_ms"`
	}

	var results []result

	if req.Feature != "" {
		for _, pc := range g.fedBridge.FindByFeature(req.Feature) {
			results = append(results, result{
				PeerID:   string(pc.PeerID),
				AgentID:  pc.Payload.AgentID,
				Models:   pc.Payload.Models,
				Adapters: pc.Payload.Adapters,
				Intents:  pc.Payload.Intents,
				Healthy:  pc.Healthy,
				Latency:  pc.Latency.Milliseconds(),
			})
		}
	} else if req.Adapter != "" {
		for _, pc := range g.fedBridge.FindByAdapter(req.Adapter) {
			results = append(results, result{
				PeerID:   string(pc.PeerID),
				AgentID:  pc.Payload.AgentID,
				Models:   pc.Payload.Models,
				Adapters: pc.Payload.Adapters,
				Intents:  pc.Payload.Intents,
				Healthy:  pc.Healthy,
				Latency:  pc.Latency.Milliseconds(),
			})
		}
	} else if req.Intent != "" {
		for _, pc := range g.fedBridge.FindByIntent(req.Intent) {
			results = append(results, result{
				PeerID:   string(pc.PeerID),
				AgentID:  pc.Payload.AgentID,
				Models:   pc.Payload.Models,
				Adapters: pc.Payload.Adapters,
				Intents:  pc.Payload.Intents,
				Healthy:  pc.Healthy,
				Latency:  pc.Latency.Milliseconds(),
			})
		}
	} else if req.MinTier != "" || len(req.Features) > 0 {
		routeResults := g.fedBridge.Route(opp.ModelRequirements{
			MinTier:  req.MinTier,
			Features: req.Features,
		})
		for _, rr := range routeResults {
			results = append(results, result{
				PeerID:  string(rr.PeerID),
				AgentID: rr.AgentID,
				Healthy: true,
			})
		}
	}

	if results == nil {
		results = []result{}
	}
	json.NewEncoder(w).Encode(map[string]any{"results": results, "count": len(results)})
}

// handleFedDelegate manually delegates a task via the federation bridge.
func (g *Gateway) handleFedDelegate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if g.fedBridge == nil || g.fedTransport == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation not configured")
		return
	}

	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var dp opp.DelegatePayload
	if err := json.NewDecoder(r.Body).Decode(&dp); err != nil {
		apperror.WriteCode(w, apperror.CodeMissingField, "invalid delegate payload")
		return
	}

	if err := dp.Validate(); err != nil {
		apperror.WriteCode(w, apperror.CodeMissingField, err.Error())
		return
	}

	result, err := g.fedBridge.Delegate(r.Context(), g.fedTransport, dp, 30*time.Second)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"status": "delegated",
		"result": result,
	})
}

// handleFedBridgeStats returns OPP bridge statistics (model-aware federation).
func (g *Gateway) handleFedBridgeStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.fedBridge == nil {
		json.NewEncoder(w).Encode(map[string]any{"configured": false})
		return
	}

	stats := g.fedBridge.Stats()
	stats["configured"] = true

	if g.fedHub != nil {
		hubStats := g.fedHub.Stats()
		for k, v := range hubStats {
			stats["hub_"+k] = v
		}
	}

	json.NewEncoder(w).Encode(stats)
}

// handleFedBroadcast triggers a capability broadcast to all known peers.
func (g *Gateway) handleFedBroadcast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if g.fedBridge == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation bridge not configured")
		return
	}

	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	g.fedBridge.BroadcastCapabilities(r.Context())
	json.NewEncoder(w).Encode(map[string]string{"status": "broadcasted"})
}
