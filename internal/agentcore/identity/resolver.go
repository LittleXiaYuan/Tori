package identity

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Profile represents a unified user identity across channels.
type Profile struct {
	UnifiedID    string            `json:"unified_id"`
	DisplayName  string            `json:"display_name"`
	Channels     map[string]string `json:"channels"`
	Metadata     map[string]string `json:"metadata"`
	FirstSeen    time.Time         `json:"first_seen"`
	LastSeen     time.Time         `json:"last_seen"`
	MessageCount int64             `json:"message_count"`
}

type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// Resolver maps channel-specific user IDs to unified profiles.
// This allows the same person on Telegram and Feishu to share memory/context.
type Resolver struct {
	mu       sync.RWMutex
	profiles map[string]*Profile // unified_id -> profile
	index    map[string]string   // "telegram:12345" -> unified_id
	kvs      kvStore
	dirty    int
}

// NewResolver creates an identity resolver.
func NewResolver() *Resolver {
	return &Resolver{
		profiles: make(map[string]*Profile),
		index:    make(map[string]string),
	}
}

// SetKVStore sets the KV store and loads persisted profiles.
func (r *Resolver) SetKVStore(kvs kvStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kvs = kvs

	var profiles map[string]*Profile
	if found, err := kvs.Get(context.Background(), "profiles", &profiles); err == nil && found {
		for id, p := range profiles {
			r.profiles[id] = p
			for ch, uid := range p.Channels {
				r.index[channelKey(ch, uid)] = id
			}
		}
		slog.Info("identity: loaded profiles from KV", "count", len(profiles))
	}
}

// FlushToKV persists all profiles to the KV store.
func (r *Resolver) FlushToKV() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.kvs == nil || r.dirty == 0 {
		return
	}
	_ = r.kvs.Put(context.Background(), "profiles", r.profiles)
	r.dirty = 0
}

func (r *Resolver) persistKV() {
	if r.kvs == nil {
		return
	}
	r.dirty++
	if r.dirty%5 == 0 {
		_ = r.kvs.Put(context.Background(), "profiles", r.profiles)
	}
}

// channelKey builds a lookup key from channel type and user ID.
func channelKey(channelType, userID string) string {
	return channelType + ":" + userID
}

// Resolve finds or creates a unified profile for a channel user.
// If the user is new, a profile is created with the channel binding.
func (r *Resolver) Resolve(channelType, userID, displayName string) *Profile {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := channelKey(channelType, userID)

	// Known user?
	if uid, ok := r.index[key]; ok {
		p := r.profiles[uid]
		p.LastSeen = time.Now()
		p.MessageCount++
		if displayName != "" && p.DisplayName == "" {
			p.DisplayName = displayName
		}
		return p.snapshot()
	}

	// New user - create profile
	uid := "u_" + userID + "_" + channelType[:2]
	p := &Profile{
		UnifiedID:   uid,
		DisplayName: displayName,
		Channels:    map[string]string{channelType: userID},
		Metadata:    map[string]string{},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		MessageCount: 1,
	}
	r.profiles[uid] = p
	r.index[key] = uid
	r.persistKV()

	return p.snapshot()
}

// Link binds an additional channel identity to an existing profile.
// Use case: user says "I'm also @xxx on Telegram" from Feishu.
func (r *Resolver) Link(unifiedID, channelType, userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.profiles[unifiedID]
	if !ok {
		return false
	}

	key := channelKey(channelType, userID)

	// Already linked to someone else?
	if existing, ok := r.index[key]; ok && existing != unifiedID {
		return false
	}

	p.Channels[channelType] = userID
	r.index[key] = unifiedID
	r.persistKV()
	return true
}

// Merge combines two profiles into one (when we discover they're the same person).
func (r *Resolver) Merge(keepID, mergeID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	keep, ok1 := r.profiles[keepID]
	merge, ok2 := r.profiles[mergeID]
	if !ok1 || !ok2 || keepID == mergeID {
		return false
	}

	// Move all channel bindings from merge to keep
	for ch, uid := range merge.Channels {
		keep.Channels[ch] = uid
		r.index[channelKey(ch, uid)] = keepID
	}
	for k, v := range merge.Metadata {
		if _, exists := keep.Metadata[k]; !exists {
			keep.Metadata[k] = v
		}
	}
	keep.MessageCount += merge.MessageCount
	if merge.FirstSeen.Before(keep.FirstSeen) {
		keep.FirstSeen = merge.FirstSeen
	}

	delete(r.profiles, mergeID)
	r.persistKV()
	return true
}

// Get returns a profile by unified ID.
func (r *Resolver) Get(unifiedID string) (*Profile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.profiles[unifiedID]
	if !ok {
		return nil, false
	}
	return p.snapshot(), true
}

// Lookup finds a profile by channel identity.
func (r *Resolver) Lookup(channelType, userID string) (*Profile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	uid, ok := r.index[channelKey(channelType, userID)]
	if !ok {
		return nil, false
	}
	p, ok := r.profiles[uid]
	if !ok {
		return nil, false
	}
	return p.snapshot(), true
}

// SetMeta sets metadata on a profile.
func (r *Resolver) SetMeta(unifiedID, key, value string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.profiles[unifiedID]
	if !ok {
		return false
	}
	p.Metadata[key] = value
	r.persistKV()
	return true
}

// All returns all profiles.
func (r *Resolver) All() []Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Profile, 0, len(r.profiles))
	for _, p := range r.profiles {
		out = append(out, *p.snapshot())
	}
	return out
}

// Count returns total profile count.
func (r *Resolver) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.profiles)
}

func (p *Profile) snapshot() *Profile {
	cp := *p
	cp.Channels = make(map[string]string, len(p.Channels))
	for k, v := range p.Channels {
		cp.Channels[k] = v
	}
	cp.Metadata = make(map[string]string, len(p.Metadata))
	for k, v := range p.Metadata {
		cp.Metadata[k] = v
	}
	return &cp
}
