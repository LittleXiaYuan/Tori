package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"yunque-agent/pkg/opp"
)

// PeerCapabilities stores the full OPP capabilities payload for a remote peer,
// including model stack, LoRA adapters, and supported intents.
type PeerCapabilities struct {
	PeerID   PeerID                `json:"peer_id"`
	Payload  opp.CapabilitiesPayload `json:"payload"`
	LastSeen time.Time             `json:"last_seen"`
	Latency  time.Duration         `json:"latency_ms"`
	Healthy  bool                  `json:"healthy"`
}

// OPPBridge bridges the federation Hub with the OPP v3 protocol.
// It maintains a model-aware capability registry and provides discovery,
// delegation, and feedback routing across federated agents.
type OPPBridge struct {
	mu   sync.RWMutex
	hub  *Hub
	caps map[PeerID]*PeerCapabilities

	// Local agent's capabilities (advertised to peers)
	localCaps opp.CapabilitiesPayload

	// Delegation handler: called when this agent receives a delegated task
	delegateHandler func(ctx context.Context, dp opp.DelegatePayload) (*opp.DelegateResultPayload, error)

	// Feedback collector: called when feedback arrives
	feedbackHandler func(ctx context.Context, fp opp.FeedbackPayload)
}

// NewOPPBridge creates a bridge that connects the federation Hub to OPP message handling.
func NewOPPBridge(hub *Hub, localCaps opp.CapabilitiesPayload) *OPPBridge {
	b := &OPPBridge{
		hub:       hub,
		caps:      make(map[PeerID]*PeerCapabilities),
		localCaps: localCaps,
	}
	b.registerHandlers()
	return b
}

// SetDelegateHandler sets the function called when this agent receives a delegated task.
func (b *OPPBridge) SetDelegateHandler(fn func(ctx context.Context, dp opp.DelegatePayload) (*opp.DelegateResultPayload, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.delegateHandler = fn
}

// SetFeedbackHandler sets the function called when feedback is received.
func (b *OPPBridge) SetFeedbackHandler(fn func(ctx context.Context, fp opp.FeedbackPayload)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.feedbackHandler = fn
}

// UpdateLocalCaps updates the local capabilities (call after LoRA adapter changes).
func (b *OPPBridge) UpdateLocalCaps(caps opp.CapabilitiesPayload) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.localCaps = caps
}

// LocalCaps returns the current local capabilities payload.
func (b *OPPBridge) LocalCaps() opp.CapabilitiesPayload {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.localCaps
}

// registerHandlers wires OPP message types into the federation Hub's handler map.
func (b *OPPBridge) registerHandlers() {
	b.hub.RegisterHandler(MsgDiscover, b.handleDiscover)
	b.hub.RegisterHandler(MsgCapReply, b.handleCapReply)
	b.hub.RegisterHandler(MsgTask, b.handleTask)
	b.hub.RegisterHandler(MsgResult, b.handleResult)
}

// handleDiscover responds with our full OPP capabilities payload.
func (b *OPPBridge) handleDiscover(ctx context.Context, msg Message) (*Message, error) {
	b.mu.RLock()
	capsJSON, _ := json.Marshal(b.localCaps)
	b.mu.RUnlock()

	return &Message{
		ID:        fmt.Sprintf("opp_%d", time.Now().UnixNano()),
		Type:      MsgCapReply,
		From:      b.hub.LocalID(),
		To:        msg.From,
		ReplyTo:   msg.ID,
		Payload:   string(capsJSON),
		Timestamp: time.Now(),
		TTL:       3,
	}, nil
}

// handleCapReply processes a peer's capabilities announcement and stores it.
func (b *OPPBridge) handleCapReply(ctx context.Context, msg Message) (*Message, error) {
	var caps opp.CapabilitiesPayload
	if err := json.Unmarshal([]byte(msg.Payload), &caps); err != nil {
		slog.Warn("opp_bridge: bad cap_reply payload", "from", msg.From, "err", err)
		return nil, nil
	}

	b.mu.Lock()
	b.caps[msg.From] = &PeerCapabilities{
		PeerID:   msg.From,
		Payload:  caps,
		LastSeen: time.Now(),
		Healthy:  true,
	}
	b.mu.Unlock()

	slog.Info("opp_bridge: peer capabilities updated",
		"peer", msg.From,
		"models", len(caps.Models),
		"adapters", len(caps.Adapters),
		"intents", len(caps.Intents))
	return nil, nil
}

// handleTask processes a delegated task from a remote peer.
func (b *OPPBridge) handleTask(ctx context.Context, msg Message) (*Message, error) {
	b.mu.RLock()
	handler := b.delegateHandler
	b.mu.RUnlock()

	if handler == nil {
		return &Message{
			ID: fmt.Sprintf("opp_%d", time.Now().UnixNano()),
			Type: MsgResult, From: b.hub.LocalID(), To: msg.From,
			ReplyTo: msg.ID, Payload: `{"error":"delegation not supported"}`,
			Timestamp: time.Now(), TTL: 3,
		}, nil
	}

	var dp opp.DelegatePayload
	if err := json.Unmarshal([]byte(msg.Payload), &dp); err != nil {
		return nil, fmt.Errorf("decode delegate payload: %w", err)
	}

	result, err := handler(ctx, dp)
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
		return &Message{
			ID: fmt.Sprintf("opp_%d", time.Now().UnixNano()),
			Type: MsgResult, From: b.hub.LocalID(), To: msg.From,
			ReplyTo: msg.ID, Payload: string(errPayload),
			Timestamp: time.Now(), TTL: 3,
		}, nil
	}

	resultJSON, _ := json.Marshal(result)
	return &Message{
		ID: fmt.Sprintf("opp_%d", time.Now().UnixNano()),
		Type: MsgResult, From: b.hub.LocalID(), To: msg.From,
		ReplyTo: msg.ID, Payload: string(resultJSON),
		Timestamp: time.Now(), TTL: 3,
	}, nil
}

// handleResult is a no-op placeholder; results are handled synchronously via Transport.
func (b *OPPBridge) handleResult(_ context.Context, _ Message) (*Message, error) {
	return nil, nil
}

// ──────────────────────────────────────────────
// Peer Query API
// ──────────────────────────────────────────────

// ListPeerCaps returns all known peer capabilities.
func (b *OPPBridge) ListPeerCaps() []PeerCapabilities {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]PeerCapabilities, 0, len(b.caps))
	for _, c := range b.caps {
		out = append(out, *c)
	}
	return out
}

// FindByFeature returns peers whose models support a specific feature (e.g. "vision", "code").
func (b *OPPBridge) FindByFeature(feature string) []PeerCapabilities {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []PeerCapabilities
	for _, pc := range b.caps {
		if !pc.Healthy {
			continue
		}
		for _, m := range pc.Payload.Models {
			for _, f := range m.Features {
				if f == feature {
					out = append(out, *pc)
					goto next
				}
			}
		}
	next:
	}
	return out
}

// FindByAdapter returns peers that have a LoRA adapter matching the given domain.
func (b *OPPBridge) FindByAdapter(domain string) []PeerCapabilities {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []PeerCapabilities
	for _, pc := range b.caps {
		if !pc.Healthy {
			continue
		}
		for _, a := range pc.Payload.Adapters {
			if a.Domain == domain {
				out = append(out, *pc)
				break
			}
		}
	}
	return out
}

// FindByIntent returns peers that support a specific intent name.
func (b *OPPBridge) FindByIntent(intentName string) []PeerCapabilities {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []PeerCapabilities
	for _, pc := range b.caps {
		if !pc.Healthy {
			continue
		}
		for _, intent := range pc.Payload.Intents {
			if intent == intentName {
				out = append(out, *pc)
				break
			}
		}
	}
	return out
}

// ──────────────────────────────────────────────
// Model-Aware Routing
// ──────────────────────────────────────────────

// modelTierRank maps tier strings to numeric scores for comparison.
var modelTierRank = map[string]int{
	"fast":   1,
	"smart":  2,
	"expert": 3,
}

// RouteResult is the output of model-aware agent selection.
type RouteResult struct {
	PeerID  PeerID  `json:"peer_id"`
	AgentID string  `json:"agent_id"`
	Score   float64 `json:"score"`
	Model   string  `json:"model,omitempty"`
	Adapter string  `json:"adapter,omitempty"`
	Reason  string  `json:"reason"`
}

// Route selects the best peer(s) for a task based on ModelRequirements.
// Returns candidates sorted by score (highest first).
func (b *OPPBridge) Route(req opp.ModelRequirements) []RouteResult {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var results []RouteResult
	minTierRank := modelTierRank[req.MinTier]

	for _, pc := range b.caps {
		if !pc.Healthy {
			continue
		}

		bestScore := 0.0
		bestModel := ""
		bestAdapter := ""
		var reasons []string

		for _, m := range pc.Payload.Models {
			score := 0.0
			mRank := modelTierRank[m.Tier]

			if mRank < minTierRank {
				continue
			}
			score += float64(mRank) * 10

			featureHits := 0
			for _, reqF := range req.Features {
				for _, mf := range m.Features {
					if mf == reqF {
						featureHits++
						break
					}
				}
			}
			if len(req.Features) > 0 && featureHits < len(req.Features) {
				continue
			}
			score += float64(featureHits) * 5

			if req.PreferLocal && m.Local {
				score += 20
				reasons = append(reasons, "local model preferred")
			}

			if req.MaxTokens > 0 && m.MaxCtx >= req.MaxTokens {
				score += 5
			}

			if score > bestScore {
				bestScore = score
				bestModel = m.ID
			}
		}

		if bestModel == "" {
			continue
		}

		if req.PreferAdapter != "" {
			for _, a := range pc.Payload.Adapters {
				if a.Domain == req.PreferAdapter {
					bestScore += 25
					bestAdapter = a.ID
					reasons = append(reasons, fmt.Sprintf("LoRA adapter match: %s", a.Domain))
					break
				}
			}
		}

		if pc.Latency > 0 && req.MaxLatencyMs > 0 {
			if int(pc.Latency.Milliseconds()) <= req.MaxLatencyMs {
				bestScore += 10
			} else {
				bestScore -= 10
				reasons = append(reasons, "latency exceeds threshold")
			}
		}

		results = append(results, RouteResult{
			PeerID:  pc.PeerID,
			AgentID: pc.Payload.AgentID,
			Score:   bestScore,
			Model:   bestModel,
			Adapter: bestAdapter,
			Reason:  fmt.Sprintf("%v", reasons),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// Delegate sends a task to the best-matching remote agent.
// It discovers candidates via Route, then delegates via the federation transport.
func (b *OPPBridge) Delegate(ctx context.Context, transport *Transport, dp opp.DelegatePayload, timeout time.Duration) (*opp.DelegateResultPayload, error) {
	var candidates []RouteResult
	if dp.ModelRequirements != nil {
		candidates = b.Route(*dp.ModelRequirements)
	}

	if len(candidates) == 0 && len(dp.FallbackAgents) > 0 {
		for _, fb := range dp.FallbackAgents {
			candidates = append(candidates, RouteResult{PeerID: PeerID(fb), Reason: "fallback"})
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable peer found for delegation")
	}

	payload, _ := json.Marshal(dp)
	for _, c := range candidates {
		slog.Info("opp_bridge: delegating task",
			"peer", c.PeerID, "score", c.Score, "model", c.Model, "adapter", c.Adapter)

		result, err := transport.DelegateTask(ctx, c.PeerID, string(payload), timeout)
		if err != nil {
			slog.Warn("opp_bridge: delegation failed, trying next", "peer", c.PeerID, "err", err)
			continue
		}

		var dr opp.DelegateResultPayload
		if err := json.Unmarshal([]byte(result), &dr); err != nil {
			return nil, fmt.Errorf("decode delegate result: %w", err)
		}
		dr.DelegatedTo = string(c.PeerID)
		dr.ModelUsed = c.Model
		dr.AdapterUsed = c.Adapter
		return &dr, nil
	}

	return nil, fmt.Errorf("all delegation candidates failed")
}

// SendFeedback sends post-task feedback to a specific peer.
func (b *OPPBridge) SendFeedback(ctx context.Context, peerID PeerID, fp opp.FeedbackPayload) error {
	payload, _ := json.Marshal(fp)
	_, err := b.hub.Send(ctx, peerID, MsgResult, string(payload))
	return err
}

// BroadcastCapabilities announces our capabilities to all known peers.
func (b *OPPBridge) BroadcastCapabilities(ctx context.Context) {
	b.mu.RLock()
	capsJSON, _ := json.Marshal(b.localCaps)
	b.mu.RUnlock()

	peers := b.hub.ListPeers()
	for _, p := range peers {
		if _, err := b.hub.Send(ctx, p.ID, MsgCapReply, string(capsJSON)); err != nil {
			slog.Warn("opp_bridge: broadcast caps failed", "peer", p.ID, "err", err)
		}
	}
}

// Stats returns bridge statistics.
func (b *OPPBridge) Stats() map[string]any {
	b.mu.RLock()
	defer b.mu.RUnlock()

	totalModels := 0
	totalAdapters := 0
	for _, pc := range b.caps {
		totalModels += len(pc.Payload.Models)
		totalAdapters += len(pc.Payload.Adapters)
	}

	return map[string]any{
		"peers_with_caps":   len(b.caps),
		"total_models":      totalModels,
		"total_adapters":    totalAdapters,
		"local_models":      len(b.localCaps.Models),
		"local_adapters":    len(b.localCaps.Adapters),
		"local_intents":     len(b.localCaps.Intents),
	}
}
