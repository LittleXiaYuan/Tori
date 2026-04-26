package cogni

import (
	"sync"
	"time"
)

// turnCache memoizes the in-progress evaluation snapshot for a single planner
// turn so the two callbacks (BuildContext and FilterSkills) can share state
// and emit one Trace per logical request — not two.
//
// The key is a fingerprint of the incoming request and entries expire after
// `ttl`. The cache is bounded; once it exceeds 1024 entries we sweep the
// expired ones lazily.
type turnCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]*turnState
}

func newTurnCache(ttl time.Duration) *turnCache {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &turnCache{ttl: ttl, m: make(map[string]*turnState)}
}

func (c *turnCache) getOrInit(req ContextRequest, ctor func() *turnState) *turnState {
	key := turnKey(req)
	c.mu.Lock()
	defer c.mu.Unlock()

	if st, ok := c.m[key]; ok {
		if time.Since(st.created) < c.ttl {
			return st
		}
		delete(c.m, key)
	}

	st := ctor()
	c.m[key] = st

	if len(c.m) > 1024 {
		c.sweepLocked()
	}
	return st
}

func (c *turnCache) sweepLocked() {
	now := time.Now()
	for k, v := range c.m {
		if now.Sub(v.created) >= c.ttl {
			delete(c.m, k)
		}
	}
}

// turnKey computes a stable fingerprint of the request. Tags / PriorHandover
// are intentionally ignored — they very rarely vary between the two callbacks
// of one turn and including them risks splitting a single turn into two
// cache entries (which would defeat the dedup).
func turnKey(req ContextRequest) string {
	return hashMessage(req.Message) + "|" + req.TenantID + "|" + req.Channel
}
