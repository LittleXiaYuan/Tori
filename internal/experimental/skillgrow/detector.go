package skillgrow

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// kvStore abstracts Ledger KV to avoid import cycles.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// MemSearchFunc searches memory for similar past requests.
type MemSearchFunc func(ctx context.Context, query string) (int, string) // count, sample

// ProposalCallback is called when a repeated pattern is detected.
type ProposalCallback func(ctx context.Context, pattern, suggestion string)

// Pattern tracks a detected repeated behavior.
type Pattern struct {
	Query     string    `json:"query"`
	Count     int       `json:"count"`
	Sample    string    `json:"sample"`
	DetectedAt time.Time `json:"detected_at"`
	Proposed  bool      `json:"proposed"`
}

// Detector monitors user interactions for repeated patterns
// and proposes automatic skill creation.
type Detector struct {
	mu           sync.Mutex
	memSearch    MemSearchFunc
	onProposal   ProposalCallback
	threshold    int              // minimum similar queries to trigger (default 3)
	patterns     map[string]*Pattern
	cooldown     time.Duration
	kvs          kvStore
}

// NewDetector creates a skill growth detector.
func NewDetector(threshold int) *Detector {
	if threshold < 2 {
		threshold = 3
	}
	return &Detector{
		threshold: threshold,
		patterns:  make(map[string]*Pattern),
		cooldown:  24 * time.Hour,
	}
}

// SetMemSearch attaches the memory search function.
func (d *Detector) SetMemSearch(fn MemSearchFunc) { d.memSearch = fn }

// SetOnProposal attaches the callback for when a skill creation is proposed.
func (d *Detector) SetOnProposal(fn ProposalCallback) { d.onProposal = fn }

// SetKVStore enables Ledger KV-backed persistence for detected patterns.
func (d *Detector) SetKVStore(kvs kvStore) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.kvs = kvs

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var patterns map[string]*Pattern
	found, err := kvs.Get(ctx, "patterns", &patterns)
	if err != nil {
		slog.Warn("skillgrow: KV load failed", "err", err)
		return
	}
	if found && len(patterns) > 0 {
		d.patterns = patterns
		slog.Info("skillgrow: loaded patterns from Ledger KV", "count", len(patterns))
	}
}

func (d *Detector) persistKV() {
	if d.kvs == nil {
		return
	}
	snap := make(map[string]*Pattern, len(d.patterns))
	for k, v := range d.patterns {
		cp := *v
		snap[k] = &cp
	}
	kvs := d.kvs
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := kvs.Put(ctx, "patterns", snap); err != nil {
			slog.Warn("skillgrow: KV save failed", "err", err)
		}
	}()
}

// Observe checks if the given query matches a repeated pattern.
// Call this after each user interaction (e.g. in learning loop's AfterInteraction).
func (d *Detector) Observe(ctx context.Context, query string) {
	if d.memSearch == nil {
		return
	}

	count, sample := d.memSearch(ctx, query)
	if count < d.threshold {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	key := normalizeQuery(query)
	p, exists := d.patterns[key]
	if exists {
		p.Count = count
		if p.Proposed && time.Since(p.DetectedAt) < d.cooldown {
			return // already proposed recently
		}
	} else {
		p = &Pattern{
			Query:      query,
			Count:      count,
			Sample:     sample,
			DetectedAt: time.Now(),
		}
		d.patterns[key] = p
	}

	// Trigger proposal
	if d.onProposal != nil && !p.Proposed {
		p.Proposed = true
		p.DetectedAt = time.Now()
		suggestion := "我注意到你经常进行类似的操作（\"" + truncate(query, 60) + "\"），" +
			"已检测到 " + itoa(count) + " 次相似请求。要不要我把它变成一个自动化技能？"
		slog.Info("skillgrow: pattern detected", "query", key, "count", count)
		d.onProposal(ctx, key, suggestion)
	}
	d.persistKV()
}

// Patterns returns all detected patterns.
func (d *Detector) Patterns() []Pattern {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]Pattern, 0, len(d.patterns))
	for _, p := range d.patterns {
		out = append(out, *p)
	}
	return out
}

// Reset clears all tracked patterns.
func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.patterns = make(map[string]*Pattern)
}

// ── Helpers ──

func normalizeQuery(s string) string {
	r := []rune(s)
	if len(r) > 80 {
		r = r[:80]
	}
	return string(r)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
