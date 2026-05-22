package ledger

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// ── Cross-Instance Synchronization ──────────────────────────────────────────
// Provides version vectors, event log replication, and conflict resolution
// for multi-instance Ledger deployments.

// VersionVector tracks the logical clock per instance for causal ordering.
type VersionVector map[string]int64 // instanceID ???logical clock

// Merge combines two version vectors, taking the max of each component.
func (vv VersionVector) Merge(other VersionVector) VersionVector {
	result := make(VersionVector)
	for k, v := range vv {
		result[k] = v
	}
	for k, v := range other {
		if existing, ok := result[k]; !ok || v > existing {
			result[k] = v
		}
	}
	return result
}

// Increment advances the clock for the given instance.
func (vv VersionVector) Increment(instanceID string) VersionVector {
	result := make(VersionVector)
	for k, v := range vv {
		result[k] = v
	}
	result[instanceID]++
	return result
}

// HappensBefore returns true if every component of vv is <= corresponding component of other,
// and at least one is strictly less.
func (vv VersionVector) HappensBefore(other VersionVector) bool {
	allLeq := true
	atLeastOneLess := false
	for k, v := range vv {
		ov := other[k]
		if v > ov {
			allLeq = false
			break
		}
		if v < ov {
			atLeastOneLess = true
		}
	}
	if !allLeq {
		return false
	}
	// Check keys in other not in vv
	for k, ov := range other {
		if _, ok := vv[k]; !ok && ov > 0 {
			atLeastOneLess = true
		}
	}
	return atLeastOneLess
}

// Concurrent returns true if neither vv < other nor other < vv.
func (vv VersionVector) Concurrent(other VersionVector) bool {
	return !vv.HappensBefore(other) && !other.HappensBefore(vv)
}

// ── Sync Protocol Messages ──────────────────────────────────────────────────

// SyncMessageType identifies the sync protocol message type.
type SyncMessageType string

const (
	SyncPull      SyncMessageType = "pull"       // request events since version
	SyncPush      SyncMessageType = "push"       // send events to peer
	SyncAck       SyncMessageType = "ack"        // acknowledge receipt
	SyncVectorReq SyncMessageType = "vector_req" // request version vector
	SyncVectorRes SyncMessageType = "vector_res" // respond with version vector
)

// SyncMessage is the wire format for sync protocol.
type SyncMessage struct {
	Type       SyncMessageType `json:"type"`
	InstanceID string          `json:"instance_id"`
	RequestID  string          `json:"request_id"`
	Vector     VersionVector   `json:"vector,omitempty"`
	Events     []*Event        `json:"events,omitempty"`
	AfterSeq   int64           `json:"after_seq,omitempty"` // for pull
	TaskID     string          `json:"task_id,omitempty"`
	Timestamp  int64           `json:"ts"`
}

// SyncTransport abstracts the network layer for sync protocol.
type SyncTransport interface {
	// Send a sync message to a specific peer instance.
	Send(ctx context.Context, peerID string, msg *SyncMessage) error
	// OnMessage registers a handler for incoming sync messages.
	OnMessage(handler func(msg *SyncMessage))
	// ListPeers returns known peer instance IDs.
	ListPeers(ctx context.Context) ([]string, error)
	// Close releases resources.
	Close() error
}

// ── Sync Engine ─────────────────────────────────────────────────────────────

// SyncEngine manages cross-instance event log replication.
type SyncEngine struct {
	instanceID string
	backend    Backend
	bus        *EventBus
	transport  SyncTransport

	mu     sync.RWMutex
	vector VersionVector
	clock  int64 // local logical clock

	// Per-peer sync state
	peerVectors map[string]VersionVector
	peerMu      sync.RWMutex

	// Conflict resolution
	conflictPolicy SyncConflictPolicy

	stopCh   chan struct{}
	stopOnce sync.Once

	// In-memory set of seen event IDs for efficient dedup during sync.
	seenMu sync.RWMutex
	seenIDs map[string]struct{}
}

// SyncConflictPolicy determines how to resolve concurrent events on the same task.
type SyncConflictPolicy int

const (
	SyncLastWriterWins SyncConflictPolicy = iota // use timestamp
	SyncHigherSeqWins                            // use higher sequence number
	SyncMergeAll                                 // attempt merge
)

// SyncEngineConfig configures the sync engine.
type SyncEngineConfig struct {
	InstanceID     string
	Backend        Backend
	Bus            *EventBus
	Transport      SyncTransport
	ConflictPolicy SyncConflictPolicy
	SyncInterval   time.Duration // periodic sync interval (default: 30s)
}

// NewSyncEngine creates a new cross-instance sync engine.
func NewSyncEngine(cfg SyncEngineConfig) *SyncEngine {
	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = 30 * time.Second
	}

	se := &SyncEngine{
		instanceID:     cfg.InstanceID,
		backend:        cfg.Backend,
		bus:            cfg.Bus,
		transport:      cfg.Transport,
		vector:         make(VersionVector),
		peerVectors:    make(map[string]VersionVector),
		conflictPolicy: cfg.ConflictPolicy,
		stopCh:         make(chan struct{}),
		seenIDs:        make(map[string]struct{}),
	}

	// Register sync message handler
	if cfg.Transport != nil {
		cfg.Transport.OnMessage(se.handleMessage)
	}

	return se
}

// Start begins periodic sync with peers.
func (se *SyncEngine) Start(ctx context.Context, interval time.Duration) {
	if interval == 0 {
		interval = 30 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-se.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				se.syncWithPeers(ctx)
			}
		}
	}()
}

// Stop halts the sync engine. Safe for concurrent / repeated calls.
func (se *SyncEngine) Stop() {
	se.stopOnce.Do(func() { close(se.stopCh) })
}

// OnLocalEvent should be called when a local event is created.
// Updates the version vector and optionally pushes to peers.
func (se *SyncEngine) OnLocalEvent(ctx context.Context, e *Event) {
	se.mu.Lock()
	se.clock++
	se.vector = se.vector.Increment(se.instanceID)
	se.mu.Unlock()

	se.seenMu.Lock()
	se.seenIDs[e.ID] = struct{}{}
	se.seenMu.Unlock()

	// Opportunistic push to all peers
	if se.transport != nil {
		se.pushEventToPeers(ctx, e)
	}
}

// GetVersionVector returns the current version vector.
func (se *SyncEngine) GetVersionVector() VersionVector {
	se.mu.RLock()
	defer se.mu.RUnlock()
	result := make(VersionVector)
	for k, v := range se.vector {
		result[k] = v
	}
	return result
}

// ── Sync Protocol Implementation ────────────────────────────────────────────

func (se *SyncEngine) syncWithPeers(ctx context.Context) {
	if se.transport == nil {
		return
	}

	peers, err := se.transport.ListPeers(ctx)
	if err != nil {
		return
	}

	for _, peerID := range peers {
		if peerID == se.instanceID {
			continue
		}
		se.pullFromPeer(ctx, peerID)
	}
}

func (se *SyncEngine) pullFromPeer(ctx context.Context, peerID string) {
	se.peerMu.RLock()
	peerVec := se.peerVectors[peerID]
	se.peerMu.RUnlock()

	afterSeq := int64(0)
	if peerVec != nil {
		afterSeq = peerVec[peerID]
	}

	msg := &SyncMessage{
		Type:       SyncPull,
		InstanceID: se.instanceID,
		RequestID:  ulid.New(),
		AfterSeq:   afterSeq,
		Timestamp:  time.Now().UnixMilli(),
	}

	se.transport.Send(ctx, peerID, msg)
}

func (se *SyncEngine) pushEventToPeers(ctx context.Context, e *Event) {
	peers, err := se.transport.ListPeers(ctx)
	if err != nil {
		return
	}

	msg := &SyncMessage{
		Type:       SyncPush,
		InstanceID: se.instanceID,
		RequestID:  ulid.New(),
		Vector:     se.GetVersionVector(),
		Events:     []*Event{e},
		Timestamp:  time.Now().UnixMilli(),
	}

	for _, peerID := range peers {
		if peerID == se.instanceID {
			continue
		}
		se.transport.Send(ctx, peerID, msg)
	}
}

func (se *SyncEngine) handleMessage(msg *SyncMessage) {
	ctx := context.Background()

	switch msg.Type {
	case SyncPull:
		se.handlePull(ctx, msg)
	case SyncPush:
		se.handlePush(ctx, msg)
	case SyncAck:
		se.handleAck(msg)
	case SyncVectorReq:
		se.handleVectorReq(ctx, msg)
	case SyncVectorRes:
		se.handleVectorRes(msg)
	}
}

func (se *SyncEngine) handlePull(ctx context.Context, msg *SyncMessage) {
	// Query local events after the requested sequence
	events, err := se.backend.QueryEvents(ctx, EventQuery{
		Limit: 100,
	})
	if err != nil {
		return
	}

	// Filter to events after afterSeq
	var filtered []*Event
	for _, e := range events {
		if e.Seq > msg.AfterSeq {
			filtered = append(filtered, e)
		}
	}

	response := &SyncMessage{
		Type:       SyncPush,
		InstanceID: se.instanceID,
		RequestID:  msg.RequestID,
		Vector:     se.GetVersionVector(),
		Events:     filtered,
		Timestamp:  time.Now().UnixMilli(),
	}

	se.transport.Send(ctx, msg.InstanceID, response)
}

func (se *SyncEngine) handlePush(ctx context.Context, msg *SyncMessage) {
	if msg.Events == nil {
		return
	}

	// Update peer version vector
	se.peerMu.Lock()
	if msg.Vector != nil {
		se.peerVectors[msg.InstanceID] = msg.Vector
	}
	se.peerMu.Unlock()

	// Merge incoming events (O(1) dedup via in-memory seen set).
	for _, e := range msg.Events {
		se.seenMu.RLock()
		_, seen := se.seenIDs[e.ID]
		se.seenMu.RUnlock()
		if seen {
			continue
		}

		if se.shouldApply(ctx, e) {
			if err := se.backend.AppendEvent(ctx, e); err == nil {
				se.seenMu.Lock()
				se.seenIDs[e.ID] = struct{}{}
				se.seenMu.Unlock()
				se.bus.Publish(e)
			}
		}
	}

	// Merge version vector
	se.mu.Lock()
	if msg.Vector != nil {
		se.vector = se.vector.Merge(msg.Vector)
	}
	se.mu.Unlock()

	// Send ACK
	ackMsg := &SyncMessage{
		Type:       SyncAck,
		InstanceID: se.instanceID,
		RequestID:  msg.RequestID,
		Vector:     se.GetVersionVector(),
		Timestamp:  time.Now().UnixMilli(),
	}
	se.transport.Send(ctx, msg.InstanceID, ackMsg)
}

func (se *SyncEngine) handleAck(msg *SyncMessage) {
	se.peerMu.Lock()
	if msg.Vector != nil {
		se.peerVectors[msg.InstanceID] = msg.Vector
	}
	se.peerMu.Unlock()
}

func (se *SyncEngine) handleVectorReq(ctx context.Context, msg *SyncMessage) {
	response := &SyncMessage{
		Type:       SyncVectorRes,
		InstanceID: se.instanceID,
		RequestID:  msg.RequestID,
		Vector:     se.GetVersionVector(),
		Timestamp:  time.Now().UnixMilli(),
	}
	se.transport.Send(ctx, msg.InstanceID, response)
}

func (se *SyncEngine) handleVectorRes(msg *SyncMessage) {
	se.peerMu.Lock()
	se.peerVectors[msg.InstanceID] = msg.Vector
	se.peerMu.Unlock()
}

// shouldApply uses conflict policy to determine if a remote event should be applied.
func (se *SyncEngine) shouldApply(ctx context.Context, e *Event) bool {
	switch se.conflictPolicy {
	case SyncLastWriterWins:
		// Always apply ???timestamp ordering is implicit
		return true
	case SyncHigherSeqWins:
		localMaxSeq, _ := se.backend.LatestEventSeq(ctx, e.TaskID)
		return e.Seq > localMaxSeq
	case SyncMergeAll:
		// Always apply for merge policy — conflicts resolved at read time
		return true
	default:
		return true
	}
}

// ── SyncMessage JSON helpers ────────────────────────────────────────────────

// Marshal serializes a SyncMessage to JSON.
func (msg *SyncMessage) Marshal() ([]byte, error) {
	return json.Marshal(msg)
}

// UnmarshalSyncMessage deserializes a SyncMessage from JSON.
func UnmarshalSyncMessage(data []byte) (*SyncMessage, error) {
	var msg SyncMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
