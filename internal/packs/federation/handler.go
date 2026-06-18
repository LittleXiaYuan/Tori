package federationpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/federation"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/opp"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.federation"

type Gateway interface {
	FederationHub() *federation.Hub
	FederationBridge() *federation.OPPBridge
	FederationTransport() *federation.Transport
}

type Handler struct {
	hubOf       func() *federation.Hub
	bridgeOf    func() *federation.OPPBridge
	transportOf func() *federation.Transport
	host        packruntime.Host
	started     atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil, nil)
	}
	return NewProvider(gateway.FederationHub, gateway.FederationBridge, gateway.FederationTransport)
}

func NewProvider(
	hub func() *federation.Hub,
	bridge func() *federation.OPPBridge,
	transport func() *federation.Transport,
) *Handler {
	return &Handler{hubOf: hub, bridgeOf: bridge, transportOf: transport}
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("federation pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/federation/peers", Handler: h.Peers},
		{Method: http.MethodGet, Path: "/v1/federation/stats", Handler: h.Stats},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/federation/capabilities", Handler: h.Capabilities},
		{Method: http.MethodPost, Path: "/v1/federation/discover", Handler: h.Discover},
		{Method: http.MethodPost, Path: "/v1/federation/delegate", Handler: h.Delegate},
		{Method: http.MethodGet, Path: "/v1/federation/bridge/stats", Handler: h.BridgeStats},
		{Method: http.MethodPost, Path: "/v1/federation/broadcast", Handler: h.Broadcast},
		{Method: http.MethodPost, Path: "/v1/federation/receive", Handler: h.Receive, Auth: packruntime.BackendRouteAuthPassthrough},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/federation/peers", Description: "List known federation peers."},
		{Method: http.MethodGet, Path: "/v1/federation/stats", Description: "Read federation hub statistics."},
		{Method: http.MethodGet, Path: "/v1/federation/capabilities", Description: "Read local and peer OPP capabilities."},
		{Method: http.MethodPost, Path: "/v1/federation/capabilities", Description: "Update local OPP capabilities."},
		{Method: http.MethodPost, Path: "/v1/federation/discover", Description: "Discover peers matching model or intent requirements."},
		{Method: http.MethodPost, Path: "/v1/federation/delegate", Description: "Delegate a task through the federation bridge."},
		{Method: http.MethodGet, Path: "/v1/federation/bridge/stats", Description: "Read model-aware federation bridge statistics."},
		{Method: http.MethodPost, Path: "/v1/federation/broadcast", Description: "Broadcast local capabilities to known peers."},
		{Method: http.MethodPost, Path: "/v1/federation/receive", Description: "Receive signed federation protocol messages from peers."},
	}
}

func Paths() []string {
	return []string{
		"/v1/federation/peers",
		"/v1/federation/stats",
		"/v1/federation/capabilities",
		"/v1/federation/discover",
		"/v1/federation/delegate",
		"/v1/federation/bridge/stats",
		"/v1/federation/broadcast",
		"/v1/federation/receive",
	}
}

func (h *Handler) hub() *federation.Hub {
	if h.hubOf == nil {
		return nil
	}
	return h.hubOf()
}

func (h *Handler) bridge() *federation.OPPBridge {
	if h.bridgeOf == nil {
		return nil
	}
	return h.bridgeOf()
}

func (h *Handler) transport() *federation.Transport {
	if h.transportOf == nil {
		return nil
	}
	return h.transportOf()
}

func (h *Handler) Peers(w http.ResponseWriter, r *http.Request) {
	writeJSONHeader(w)
	hub := h.hub()
	if hub == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "federation not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"local_id": string(hub.LocalID()),
		"peers":    hub.ListPeers(),
	})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	writeJSONHeader(w)
	hub := h.hub()
	if hub == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "federation not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(hub.Stats())
}

func (h *Handler) Capabilities(w http.ResponseWriter, r *http.Request) {
	writeJSONHeader(w)
	bridge := h.bridge()
	if bridge == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation bridge not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		caps := bridge.LocalCaps()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"local": caps,
			"peers": bridge.ListPeerCaps(),
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
		bridge.UpdateLocalCaps(caps)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST")
	}
}

func (h *Handler) Discover(w http.ResponseWriter, r *http.Request) {
	writeJSONHeader(w)
	bridge := h.bridge()
	if bridge == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation bridge not configured")
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var req struct {
		Feature  string   `json:"feature"`
		Adapter  string   `json:"adapter"`
		Intent   string   `json:"intent"`
		MinTier  string   `json:"min_tier"`
		Features []string `json:"features"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeMissingField, "invalid request")
		return
	}

	type result struct {
		PeerID   string            `json:"peer_id"`
		AgentID  string            `json:"agent_id"`
		Models   []opp.ModelInfo   `json:"models"`
		Adapters []opp.AdapterInfo `json:"adapters"`
		Intents  []string          `json:"intents"`
		Healthy  bool              `json:"healthy"`
		Latency  int64             `json:"latency_ms"`
	}
	var results []result

	if req.Feature != "" {
		for _, pc := range bridge.FindByFeature(req.Feature) {
			results = append(results, peerCapabilityResult(pc))
		}
	} else if req.Adapter != "" {
		for _, pc := range bridge.FindByAdapter(req.Adapter) {
			results = append(results, peerCapabilityResult(pc))
		}
	} else if req.Intent != "" {
		for _, pc := range bridge.FindByIntent(req.Intent) {
			results = append(results, peerCapabilityResult(pc))
		}
	} else if req.MinTier != "" || len(req.Features) > 0 {
		routeResults := bridge.Route(opp.ModelRequirements{
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
	_ = json.NewEncoder(w).Encode(map[string]any{"results": results, "count": len(results)})
}

func peerCapabilityResult(pc federation.PeerCapabilities) struct {
	PeerID   string            `json:"peer_id"`
	AgentID  string            `json:"agent_id"`
	Models   []opp.ModelInfo   `json:"models"`
	Adapters []opp.AdapterInfo `json:"adapters"`
	Intents  []string          `json:"intents"`
	Healthy  bool              `json:"healthy"`
	Latency  int64             `json:"latency_ms"`
} {
	return struct {
		PeerID   string            `json:"peer_id"`
		AgentID  string            `json:"agent_id"`
		Models   []opp.ModelInfo   `json:"models"`
		Adapters []opp.AdapterInfo `json:"adapters"`
		Intents  []string          `json:"intents"`
		Healthy  bool              `json:"healthy"`
		Latency  int64             `json:"latency_ms"`
	}{
		PeerID:   string(pc.PeerID),
		AgentID:  pc.Payload.AgentID,
		Models:   pc.Payload.Models,
		Adapters: pc.Payload.Adapters,
		Intents:  pc.Payload.Intents,
		Healthy:  pc.Healthy,
		Latency:  pc.Latency.Milliseconds(),
	}
}

func (h *Handler) Delegate(w http.ResponseWriter, r *http.Request) {
	writeJSONHeader(w)
	bridge := h.bridge()
	transport := h.transport()
	if bridge == nil || transport == nil {
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
	result, err := bridge.Delegate(r.Context(), transport, dp, 30*time.Second)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "delegated", "result": result})
}

func (h *Handler) BridgeStats(w http.ResponseWriter, r *http.Request) {
	writeJSONHeader(w)
	bridge := h.bridge()
	if bridge == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"configured": false})
		return
	}
	stats := bridge.Stats()
	stats["configured"] = true
	if hub := h.hub(); hub != nil {
		for k, v := range hub.Stats() {
			stats["hub_"+k] = v
		}
	}
	_ = json.NewEncoder(w).Encode(stats)
}

func (h *Handler) Broadcast(w http.ResponseWriter, r *http.Request) {
	writeJSONHeader(w)
	bridge := h.bridge()
	if bridge == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation bridge not configured")
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	bridge.BroadcastCapabilities(r.Context())
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "broadcasted"})
}

func (h *Handler) Receive(w http.ResponseWriter, r *http.Request) {
	transport := h.transport()
	if transport == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "federation transport not configured")
		return
	}
	transport.HTTPHandler()(w, r)
}

func writeJSONHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}
