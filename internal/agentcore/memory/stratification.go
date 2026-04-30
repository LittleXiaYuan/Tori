package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ── Memory Stratification: L1 Raw → L2 Feature → L3 Structure → L4 Pattern ──
//
// Four-layer memory hierarchy inspired by cognitive science's sensory→working→
// declarative→procedural progression. Each layer progressively abstracts raw
// input into reusable knowledge, with async Go channel pipelines between layers.

// InputSource identifies where raw input originated.
type LayerSource string

const (
	LayerSourceChat    LayerSource = "chat"
	LayerSourceTool    LayerSource = "tool_return"
	LayerSourceMCP     LayerSource = "mcp_response"
	LayerSourceWebhook LayerSource = "webhook"
	LayerSourceSystem  LayerSource = "system"
)

// ── Data Models ──

// L1Entry is a raw, unprocessed input record.
type L1Entry struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id"`
	Source    LayerSource       `json:"source"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	Promoted  bool              `json:"promoted"`
}

// L2Entry is a fact/feature extracted from L1 raw data.
type L2Entry struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Fact      string    `json:"fact"`
	Category  string    `json:"category"` // fact / preference / knowledge / experience
	Embedding []float32 `json:"embedding,omitempty"`
	Score     float64   `json:"score"`
	SourceL1  string    `json:"source_l1"`
	AccessCnt int       `json:"access_cnt"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Promoted  bool      `json:"promoted"`
}

// PromoteRequest carries data between layer pipelines.
type PromoteRequest struct {
	TenantID string
	EntryID  string
	Content  string
	Source   LayerSource
	Metadata map[string]string
}

// ── MemoryLayer Interface ──

// MemoryLayer is the unified interface for each stratification layer.
// All four layers (L1–L4) implement this interface.
type MemoryLayer interface {
	Name() string
	Store(ctx context.Context, tenantID string, item Item) error
	Retrieve(ctx context.Context, tenantID, key string) (*Item, error)
	Search(ctx context.Context, tenantID, query string, limit int) ([]Item, error)
	Promote(ctx context.Context, tenantID, key string) error
	Demote(ctx context.Context, tenantID, key string) error
	Count(tenantID string) int
}

// ── L1 Raw Layer ──

// L1RawLayer stores all unprocessed inputs as the source of truth.
// Uses an in-memory map with optional TTL eviction.
type L1RawLayer struct {
	mu       sync.RWMutex
	entries  map[string]map[string]*L1Entry // tenantID → entryID → entry
	maxAge   time.Duration                  // entries older than this are evictable
	maxItems int                            // per-tenant cap (0 = unlimited)
}

type L1Config struct {
	MaxAge   time.Duration // default: 7 days
	MaxItems int           // default: 10000 per tenant
}

func DefaultL1Config() L1Config {
	return L1Config{
		MaxAge:   7 * 24 * time.Hour,
		MaxItems: 10_000,
	}
}

func NewL1RawLayer(cfg L1Config) *L1RawLayer {
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 7 * 24 * time.Hour
	}
	if cfg.MaxItems == 0 {
		cfg.MaxItems = 10_000
	}
	return &L1RawLayer{
		entries:  make(map[string]map[string]*L1Entry),
		maxAge:   cfg.MaxAge,
		maxItems: cfg.MaxItems,
	}
}

func (l *L1RawLayer) Name() string { return "L1_raw" }

func (l *L1RawLayer) Store(ctx context.Context, tenantID string, item Item) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.entries[tenantID] == nil {
		l.entries[tenantID] = make(map[string]*L1Entry)
	}
	if l.maxItems > 0 && len(l.entries[tenantID]) >= l.maxItems {
		l.evictOldest(tenantID)
	}

	key := item.Key
	if key == "" {
		key = uuid.New().String()
	}
	l.entries[tenantID][key] = &L1Entry{
		ID:        key,
		TenantID:  tenantID,
		Source:    LayerSource(item.Source),
		Content:   item.Value,
		Metadata:  map[string]string{"category": item.Category},
		CreatedAt: time.Now(),
	}
	return nil
}

func (l *L1RawLayer) Retrieve(_ context.Context, tenantID, key string) (*Item, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	tenant, ok := l.entries[tenantID]
	if !ok {
		return nil, fmt.Errorf("tenant %q not found in L1", tenantID)
	}
	entry, ok := tenant[key]
	if !ok {
		return nil, fmt.Errorf("entry %q not found in L1", key)
	}
	return &Item{
		ID:        entry.ID,
		Key:       entry.ID,
		Value:     entry.Content,
		Source:    string(entry.Source),
		CreatedAt: entry.CreatedAt,
	}, nil
}

func (l *L1RawLayer) Search(_ context.Context, tenantID, query string, limit int) ([]Item, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	tenant, ok := l.entries[tenantID]
	if !ok {
		return nil, nil
	}

	var results []Item
	for _, entry := range tenant {
		if containsIgnoreCase(entry.Content, query) {
			results = append(results, Item{
				ID:        entry.ID,
				Key:       entry.ID,
				Value:     entry.Content,
				Source:    string(entry.Source),
				CreatedAt: entry.CreatedAt,
			})
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (l *L1RawLayer) Promote(_ context.Context, tenantID, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if tenant, ok := l.entries[tenantID]; ok {
		if entry, ok := tenant[key]; ok {
			entry.Promoted = true
			return nil
		}
	}
	return fmt.Errorf("entry %q not found in L1 for tenant %q", key, tenantID)
}

func (l *L1RawLayer) Demote(_ context.Context, _, _ string) error {
	return nil // L1 is the bottom layer; demote is a no-op
}

func (l *L1RawLayer) Count(tenantID string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries[tenantID])
}

func (l *L1RawLayer) evictOldest(tenantID string) {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range l.entries[tenantID] {
		if entry.Promoted {
			continue // don't evict promoted entries prematurely
		}
		if first || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(l.entries[tenantID], oldestKey)
	}
}

// ── L2 Feature Layer ──

// L2FeatureLayer stores extracted facts with optional embeddings.
// Builds on top of the existing memory.Store interface for search.
type L2FeatureLayer struct {
	mu      sync.RWMutex
	entries map[string]map[string]*L2Entry // tenantID → entryID → entry
	embedFn EmbedFunc                      // optional: generate embeddings
}

func NewL2FeatureLayer(embedFn EmbedFunc) *L2FeatureLayer {
	return &L2FeatureLayer{
		entries: make(map[string]map[string]*L2Entry),
		embedFn: embedFn,
	}
}

func (l *L2FeatureLayer) Name() string { return "L2_feature" }

func (l *L2FeatureLayer) Store(ctx context.Context, tenantID string, item Item) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.entries[tenantID] == nil {
		l.entries[tenantID] = make(map[string]*L2Entry)
	}

	key := item.Key
	if key == "" {
		key = uuid.New().String()
	}

	entry := &L2Entry{
		ID:       key,
		TenantID: tenantID,
		Fact:     item.Value,
		Category: item.Category,
		Score:    item.Score,
		SourceL1: item.Source,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if l.embedFn != nil {
		if vecs, err := l.embedFn(ctx, []string{item.Value}); err == nil && len(vecs) > 0 {
			entry.Embedding = vecs[0]
		}
	}

	l.entries[tenantID][key] = entry
	return nil
}

func (l *L2FeatureLayer) Retrieve(_ context.Context, tenantID, key string) (*Item, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	tenant, ok := l.entries[tenantID]
	if !ok {
		return nil, fmt.Errorf("tenant %q not found in L2", tenantID)
	}
	entry, ok := tenant[key]
	if !ok {
		return nil, fmt.Errorf("entry %q not found in L2", key)
	}

	entry.AccessCnt++
	entry.UpdatedAt = time.Now()

	return &Item{
		ID:        entry.ID,
		Key:       entry.ID,
		Value:     entry.Fact,
		Source:    entry.SourceL1,
		Category:  entry.Category,
		Score:     entry.Score,
		Embedding: entry.Embedding,
		AccessCnt: entry.AccessCnt,
		CreatedAt: entry.CreatedAt,
	}, nil
}

func (l *L2FeatureLayer) Search(_ context.Context, tenantID, query string, limit int) ([]Item, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	tenant, ok := l.entries[tenantID]
	if !ok {
		return nil, nil
	}

	var results []Item
	for _, entry := range tenant {
		if containsIgnoreCase(entry.Fact, query) {
			results = append(results, Item{
				ID:        entry.ID,
				Key:       entry.ID,
				Value:     entry.Fact,
				Source:    entry.SourceL1,
				Category:  entry.Category,
				Score:     entry.Score,
				Embedding: entry.Embedding,
				AccessCnt: entry.AccessCnt,
				CreatedAt: entry.CreatedAt,
			})
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	// If embeddings are available and we have an embed function, do vector search
	if l.embedFn != nil && len(results) == 0 {
		results = l.vectorSearch(tenantID, query, limit)
	}

	return results, nil
}

func (l *L2FeatureLayer) vectorSearch(tenantID, query string, limit int) []Item {
	vecs, err := l.embedFn(context.Background(), []string{query})
	if err != nil || len(vecs) == 0 {
		return nil
	}
	queryEmbed := vecs[0]
	if len(queryEmbed) == 0 {
		return nil
	}

	type scored struct {
		item  Item
		score float64
	}
	var candidates []scored

	tenant := l.entries[tenantID]
	for _, entry := range tenant {
		if len(entry.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(queryEmbed, entry.Embedding)
		candidates = append(candidates, scored{
			item: Item{
				ID:       entry.ID,
				Key:      entry.ID,
				Value:    entry.Fact,
				Source:   entry.SourceL1,
				Category: entry.Category,
				Score:    sim,
			},
			score: sim,
		})
	}

	// Sort by similarity descending
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]Item, len(candidates))
	for i, c := range candidates {
		results[i] = c.item
	}
	return results
}

func (l *L2FeatureLayer) Promote(_ context.Context, tenantID, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if tenant, ok := l.entries[tenantID]; ok {
		if entry, ok := tenant[key]; ok {
			entry.Promoted = true
			return nil
		}
	}
	return fmt.Errorf("entry %q not found in L2 for tenant %q", key, tenantID)
}

func (l *L2FeatureLayer) Demote(ctx context.Context, tenantID, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if tenant, ok := l.entries[tenantID]; ok {
		if _, ok := tenant[key]; ok {
			delete(tenant, key)
			return nil
		}
	}
	return fmt.Errorf("entry %q not found in L2 for tenant %q", key, tenantID)
}

func (l *L2FeatureLayer) Count(tenantID string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries[tenantID])
}

// ── Layer Pipeline Manager ──

// LayerPipeline manages async data flow between memory layers.
// Uses buffered Go channels with backpressure and batch promotion.
type LayerPipeline struct {
	l1     MemoryLayer
	l2     MemoryLayer

	l1ToL2 chan PromoteRequest

	extractFn   func(ctx context.Context, content string) ([]string, error)
	importanceFn func(ctx context.Context, content string) float64

	batchSize    int
	batchTimeout time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup

	// Metrics
	mu          sync.Mutex
	promoted    int64
	dropped     int64
	errors      int64
}

// LayerPipelineConfig tunes the async pipeline.
type LayerPipelineConfig struct {
	ChannelBuffer int           // default: 256
	BatchSize     int           // default: 10
	BatchTimeout  time.Duration // default: 30s
	Workers       int           // default: 2
}

func DefaultLayerPipelineConfig() LayerPipelineConfig {
	return LayerPipelineConfig{
		ChannelBuffer: 256,
		BatchSize:     10,
		BatchTimeout:  30 * time.Second,
		Workers:       2,
	}
}

// NewLayerPipeline creates a pipeline for L1→L2 promotion.
// extractFn extracts facts from raw content (can be LLM-backed or rule-based).
// importanceFn scores a fact's importance (0.0 - 1.0).
func NewLayerPipeline(
	l1, l2 MemoryLayer,
	extractFn func(ctx context.Context, content string) ([]string, error),
	importanceFn func(ctx context.Context, content string) float64,
	cfg LayerPipelineConfig,
) *LayerPipeline {
	if cfg.ChannelBuffer == 0 {
		cfg.ChannelBuffer = 256
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	if cfg.BatchTimeout == 0 {
		cfg.BatchTimeout = 30 * time.Second
	}
	if cfg.Workers == 0 {
		cfg.Workers = 2
	}

	lp := &LayerPipeline{
		l1:           l1,
		l2:           l2,
		l1ToL2:       make(chan PromoteRequest, cfg.ChannelBuffer),
		extractFn:    extractFn,
		importanceFn: importanceFn,
		batchSize:    cfg.BatchSize,
		batchTimeout: cfg.BatchTimeout,
		stopCh:       make(chan struct{}),
	}

	for i := 0; i < cfg.Workers; i++ {
		lp.wg.Add(1)
		go lp.l1ToL2Worker(i)
	}

	return lp
}

// Submit enqueues a raw entry for L1→L2 promotion.
// Non-blocking: drops the request if the channel is full.
func (lp *LayerPipeline) Submit(req PromoteRequest) bool {
	select {
	case lp.l1ToL2 <- req:
		return true
	default:
		lp.mu.Lock()
		lp.dropped++
		lp.mu.Unlock()
		slog.Warn("stratification: L1→L2 channel full, dropping request",
			"tenant", req.TenantID, "entry", req.EntryID)
		return false
	}
}

// Stop gracefully shuts down the pipeline.
func (lp *LayerPipeline) Stop() {
	close(lp.stopCh)
	lp.wg.Wait()
}

// Stats returns pipeline throughput metrics.
func (lp *LayerPipeline) Stats() (promoted, dropped, errors int64) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	return lp.promoted, lp.dropped, lp.errors
}

func (lp *LayerPipeline) l1ToL2Worker(workerID int) {
	defer lp.wg.Done()

	batch := make([]PromoteRequest, 0, lp.batchSize)
	timer := time.NewTimer(lp.batchTimeout)
	defer timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		for _, req := range batch {
			lp.processL1ToL2(ctx, req)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-lp.stopCh:
			flush()
			return

		case req := <-lp.l1ToL2:
			batch = append(batch, req)
			if len(batch) >= lp.batchSize {
				flush()
				timer.Reset(lp.batchTimeout)
			}

		case <-timer.C:
			flush()
			timer.Reset(lp.batchTimeout)
		}
	}
}

func (lp *LayerPipeline) processL1ToL2(ctx context.Context, req PromoteRequest) {
	if lp.extractFn == nil {
		lp.storeDirectFact(ctx, req)
		return
	}

	facts, err := lp.extractFn(ctx, req.Content)
	if err != nil {
		slog.Warn("stratification: fact extraction failed, storing raw",
			"err", err, "tenant", req.TenantID)
		lp.storeDirectFact(ctx, req)
		lp.mu.Lock()
		lp.errors++
		lp.mu.Unlock()
		return
	}

	for _, fact := range facts {
		score := 0.5
		if lp.importanceFn != nil {
			score = lp.importanceFn(ctx, fact)
		}

		err := lp.l2.Store(ctx, req.TenantID, Item{
			Key:      uuid.New().String(),
			Value:    fact,
			Source:   req.EntryID,
			Category: "fact",
			Score:    score,
		})
		if err != nil {
			slog.Warn("stratification: L2 store failed", "err", err)
			lp.mu.Lock()
			lp.errors++
			lp.mu.Unlock()
			continue
		}

		lp.mu.Lock()
		lp.promoted++
		lp.mu.Unlock()
	}

	// Mark L1 entry as promoted
	_ = lp.l1.Promote(ctx, req.TenantID, req.EntryID)
}

func (lp *LayerPipeline) storeDirectFact(ctx context.Context, req PromoteRequest) {
	score := 0.5
	if lp.importanceFn != nil {
		score = lp.importanceFn(ctx, req.Content)
	}
	err := lp.l2.Store(ctx, req.TenantID, Item{
		Key:      uuid.New().String(),
		Value:    req.Content,
		Source:   req.EntryID,
		Category: "raw_fact",
		Score:    score,
	})
	if err != nil {
		lp.mu.Lock()
		lp.errors++
		lp.mu.Unlock()
	} else {
		lp.mu.Lock()
		lp.promoted++
		lp.mu.Unlock()
	}
}

// ── Helpers ──

func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	sLower := make([]byte, len(s))
	subLower := make([]byte, len(substr))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		sLower[i] = c
	}
	for i := 0; i < len(substr); i++ {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		subLower[i] = c
	}
	for i := 0; i <= len(sLower)-len(subLower); i++ {
		match := true
		for j := 0; j < len(subLower); j++ {
			if sLower[i+j] != subLower[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// cosineSimilarity and sqrt64 are defined in long.go; reused here.
