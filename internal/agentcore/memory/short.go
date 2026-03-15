package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// session holds a sliding window of items for one tenant+session.
type session struct {
	items     []Item
	lastTouch time.Time
}

// ShortTerm is a session-scoped sliding window memory.
// Each tenant gets independent sessions. Items are ordered by insertion time.
// Oldest items are evicted when the window exceeds maxPerSession.
type ShortTerm struct {
	mu            sync.RWMutex
	sessions      map[string]*session // "tenantID:sessionKey" -> session
	ttl           time.Duration       // session expiry after last touch
	maxPerSession int                 // max items per session (sliding window)
}

// NewShortTerm creates a short-term memory with the given TTL.
func NewShortTerm(ttl time.Duration) *ShortTerm {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &ShortTerm{
		sessions:      make(map[string]*session),
		ttl:           ttl,
		maxPerSession: 50,
	}
}

func sessionKey(tenantID, key string) string {
	if key == "" {
		return tenantID + ":_default"
	}
	return tenantID + ":" + key
}

func (s *ShortTerm) Put(_ context.Context, tenantID string, item Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	item.ExpiresAt = time.Now().Add(s.ttl)

	sk := sessionKey(tenantID, item.Key)
	sess, ok := s.sessions[sk]
	if !ok {
		sess = &session{}
		s.sessions[sk] = sess
	}
	sess.lastTouch = time.Now()
	sess.items = append(sess.items, item)

	// Sliding window: evict oldest if over limit
	if len(sess.items) > s.maxPerSession {
		sess.items = sess.items[len(sess.items)-s.maxPerSession:]
	}
	return nil
}

func (s *ShortTerm) Get(_ context.Context, tenantID, key string) (*Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sk := sessionKey(tenantID, key)
	sess := s.sessions[sk]
	if sess == nil || len(sess.items) == 0 {
		return nil, nil
	}
	if time.Now().After(sess.lastTouch.Add(s.ttl)) {
		return nil, nil
	}
	last := sess.items[len(sess.items)-1]
	return &last, nil
}

func (s *ShortTerm) Search(_ context.Context, tenantID, query string, limit int) ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	prefix := tenantID + ":"
	queryLower := strings.ToLower(query)

	var results []Item
	for sk, sess := range s.sessions {
		if !strings.HasPrefix(sk, prefix) {
			continue
		}
		if now.After(sess.lastTouch.Add(s.ttl)) {
			continue
		}
		// Search in reverse order (most recent first)
		for i := len(sess.items) - 1; i >= 0; i-- {
			item := sess.items[i]
			if query == "" || strings.Contains(strings.ToLower(item.Value), queryLower) ||
				strings.Contains(strings.ToLower(item.Key), queryLower) {
				// Recency score: newer items score higher (0.5 - 1.0)
				age := now.Sub(item.CreatedAt).Seconds()
				maxAge := s.ttl.Seconds()
				if maxAge <= 0 {
					maxAge = 1
				}
				item.Score = 1.0 - 0.5*(age/maxAge)
				if item.Score < 0.1 {
					item.Score = 0.1
				}
				results = append(results, item)
			}
			if limit > 0 && len(results) >= limit {
				break
			}
		}
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	// Sort by score descending (most recent/relevant first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *ShortTerm) Delete(_ context.Context, tenantID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionKey(tenantID, key))
	return nil
}

func (s *ShortTerm) List(_ context.Context, tenantID, prefix string, limit int) ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	tPrefix := tenantID + ":"
	var results []Item
	for sk, sess := range s.sessions {
		if !strings.HasPrefix(sk, tPrefix) {
			continue
		}
		if now.After(sess.lastTouch.Add(s.ttl)) {
			continue
		}
		for _, item := range sess.items {
			if prefix == "" || strings.HasPrefix(item.Key, prefix) {
				results = append(results, item)
			}
			if limit > 0 && len(results) >= limit {
				return results, nil
			}
		}
	}
	return results, nil
}

// Count returns total items across all active sessions for a tenant.
func (s *ShortTerm) Count(tenantID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	prefix := tenantID + ":"
	total := 0
	for sk, sess := range s.sessions {
		if strings.HasPrefix(sk, prefix) && !now.After(sess.lastTouch.Add(s.ttl)) {
			total += len(sess.items)
		}
	}
	return total
}

// GC removes expired sessions. Call periodically.
func (s *ShortTerm) GC() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for sk, sess := range s.sessions {
		if now.After(sess.lastTouch.Add(s.ttl)) {
			delete(s.sessions, sk)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
