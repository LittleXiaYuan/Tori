package cogni

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"
)

// CogniBus broadcasts user intents to all registered Cognis and selects
// the best handler(s) via a competitive bidding mechanism.
//
// Flow:
//  1. User message arrives → Broadcast to all active Cognis
//  2. Each Cogni returns a Bid (confidence, cost estimate, ETA)
//  3. Bus ranks bids and selects winner(s)
//  4. Selected Cogni(s) handle the request
type CogniBus struct {
	mu        sync.RWMutex
	evaluator *Evaluator
	cognis    map[string]*Declaration
	bidders   map[string]Bidder // optional: cognis that support bidding
	cfg       BusConfig
}

// BusConfig controls bidding behavior.
type BusConfig struct {
	MaxConcurrent int           // max cognis that can handle a single request (default 1)
	BidTimeout    time.Duration // max time to wait for bids (default 500ms)
	MinConfidence float64       // minimum bid confidence to consider (default 0.3)
}

// DefaultBusConfig returns production defaults.
func DefaultBusConfig() BusConfig {
	return BusConfig{
		MaxConcurrent: 1,
		BidTimeout:    500 * time.Millisecond,
		MinConfidence: 0.3,
	}
}

// Bid is a Cogni's response to an intent broadcast.
type Bid struct {
	CogniID    string        `json:"cogni_id"`
	Confidence float64       `json:"confidence"` // 0-1
	Cost       float64       `json:"cost"`       // estimated token/resource cost
	ETA        time.Duration `json:"eta"`        // estimated time to complete
	Reason     string        `json:"reason"`
}

// Bidder is an optional interface Cognis can implement for custom bidding
// logic beyond the standard activation scoring.
type Bidder interface {
	Bid(ctx context.Context, session Session) (*Bid, error)
}

// RouteResult is the output of the bidding process.
type RouteResult struct {
	Winners     []Bid         `json:"winners"`
	AllBids     []Bid         `json:"all_bids"`
	SelectedIDs []string      `json:"selected_ids"`
	Duration    time.Duration `json:"duration"`
}

func NewCogniBus(evaluator *Evaluator, cfg BusConfig) *CogniBus {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 1
	}
	return &CogniBus{
		evaluator: evaluator,
		cognis:    make(map[string]*Declaration),
		bidders:   make(map[string]Bidder),
		cfg:       cfg,
	}
}

// Register adds a Cogni to the bus.
func (b *CogniBus) Register(d *Declaration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cognis[d.ID] = d
}

// RegisterBidder attaches a custom bidder to a Cogni.
func (b *CogniBus) RegisterBidder(cogniID string, bidder Bidder) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bidders[cogniID] = bidder
}

// Unregister removes a Cogni from the bus.
func (b *CogniBus) Unregister(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.cognis, id)
	delete(b.bidders, id)
}

// Route broadcasts a session to all Cognis and returns ranked results.
func (b *CogniBus) Route(ctx context.Context, session Session) *RouteResult {
	start := time.Now()

	b.mu.RLock()
	decls := make([]*Declaration, 0, len(b.cognis))
	for _, d := range b.cognis {
		decls = append(decls, d)
	}
	bidders := make(map[string]Bidder, len(b.bidders))
	for k, v := range b.bidders {
		bidders[k] = v
	}
	b.mu.RUnlock()

	bidCtx, cancel := context.WithTimeout(ctx, b.cfg.BidTimeout)
	defer cancel()

	// Phase 1: Standard evaluation
	activations := b.evaluator.Evaluate(decls, session)

	// Phase 2: Collect bids
	var allBids []Bid
	for _, act := range activations {
		if act.Declaration == nil {
			continue
		}
		id := act.Declaration.ID

		bid := Bid{
			CogniID:    id,
			Confidence: act.Score,
			Reason:     joinReasons(act.Reasons),
		}

		// If the cogni has a custom bidder, use it for more refined bidding
		if bidder, ok := bidders[id]; ok && act.Score >= b.cfg.MinConfidence {
			customBid, err := bidder.Bid(bidCtx, session)
			if err == nil && customBid != nil {
				bid.Confidence = customBid.Confidence
				bid.Cost = customBid.Cost
				bid.ETA = customBid.ETA
				if customBid.Reason != "" {
					bid.Reason = customBid.Reason
				}
			}
		}

		if bid.Confidence >= b.cfg.MinConfidence {
			allBids = append(allBids, bid)
		}
	}

	// Phase 3: Rank and select winners
	sort.Slice(allBids, func(i, j int) bool {
		if allBids[i].Confidence != allBids[j].Confidence {
			return allBids[i].Confidence > allBids[j].Confidence
		}
		return allBids[i].Cost < allBids[j].Cost
	})

	winners := allBids
	if len(winners) > b.cfg.MaxConcurrent {
		winners = winners[:b.cfg.MaxConcurrent]
	}

	selectedIDs := make([]string, len(winners))
	for i, w := range winners {
		selectedIDs[i] = w.CogniID
	}

	result := &RouteResult{
		Winners:     winners,
		AllBids:     allBids,
		SelectedIDs: selectedIDs,
		Duration:    time.Since(start),
	}

	if len(winners) > 0 {
		slog.Debug("cogni_bus: routed",
			"winners", selectedIDs,
			"total_bids", len(allBids),
			"duration", result.Duration,
		)
	}

	return result
}

// ActiveCognis returns the count of registered Cognis.
func (b *CogniBus) ActiveCognis() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.cognis)
}

// Clear removes all registered cognis and custom bidders. Runtime owners call
// this when the Cogni Kernel pack is disabled so stale declarations no longer
// participate in background routing.
func (b *CogniBus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cognis = make(map[string]*Declaration)
	b.bidders = make(map[string]Bidder)
}

func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	if len(reasons) == 1 {
		return reasons[0]
	}
	return reasons[0] + " (+" + strconv.Itoa(len(reasons)-1) + " more)"
}
