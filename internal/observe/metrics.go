package observe

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// BreakerStatusFunc returns the circuit breaker state and failure count.
type BreakerStatusFunc func() (state string, failures int)

// Metrics collects observability data for the agent.
// Thread-safe, lock-free counters for hot paths.
type Metrics struct {
	// Request counters
	RequestsTotal   atomic.Int64
	RequestsSuccess atomic.Int64
	RequestsFailed  atomic.Int64

	// Token usage
	TokensIn  atomic.Int64
	TokensOut atomic.Int64

	// LLM breaker counters
	BreakerTrips atomic.Int64

	// Channel message counters
	channels *ChannelMetrics

	// Knowledge/RAG counters
	knowledge *KnowledgeMetrics

	// Latency tracking
	latencies *HistogramStore

	// Skill/tool call tracking
	skills *SkillMetrics

	// Error tracking
	errors *ErrorTracker

	// Breaker status function
	breakerStatus BreakerStatusFunc

	// Start time
	startTime time.Time
}

// New creates a new Metrics collector.
func New() *Metrics {
	return &Metrics{
		latencies: newHistogramStore(),
		skills:    newSkillMetrics(),
		errors:    newErrorTracker(),
		channels:  newChannelMetrics(),
		knowledge: newKnowledgeMetrics(),
		startTime: time.Now(),
	}
}

// SetBreakerStatus sets the function to query circuit breaker state.
func (m *Metrics) SetBreakerStatus(fn BreakerStatusFunc) { m.breakerStatus = fn }

// RecordBreakerTrip increments the breaker trip counter.
func (m *Metrics) RecordBreakerTrip() { m.BreakerTrips.Add(1) }

// RecordRequest records a completed request with its latency and token counts.
func (m *Metrics) RecordRequest(duration time.Duration, tokensIn, tokensOut int64, err error) {
	m.RequestsTotal.Add(1)
	if err != nil {
		m.RequestsFailed.Add(1)
		m.errors.Record(err)
	} else {
		m.RequestsSuccess.Add(1)
	}
	m.TokensIn.Add(tokensIn)
	m.TokensOut.Add(tokensOut)
	m.latencies.Record("request", duration)
}

// RecordSkillCall records a skill/tool invocation.
func (m *Metrics) RecordSkillCall(skillName string, duration time.Duration, err error) {
	m.skills.Record(skillName, duration, err)
}

// RecordChannelMessage records an incoming message from a channel.
func (m *Metrics) RecordChannelMessage(channelType string) {
	m.channels.RecordIn(channelType)
}

// RecordChannelSend records an outgoing message sent to a channel.
func (m *Metrics) RecordChannelSend(channelType string, err error) {
	m.channels.RecordOut(channelType, err)
}

// RecordKnowledgeSearch records a knowledge base search operation.
func (m *Metrics) RecordKnowledgeSearch(searchType string, duration time.Duration, results int) {
	m.knowledge.RecordSearch(searchType, duration, results)
}

// RecordRerank records a rerank operation.
func (m *Metrics) RecordRerank(provider string, duration time.Duration, err error) {
	m.knowledge.RecordRerank(provider, duration, err)
}

// RecordLLMCall records an LLM API call latency.
func (m *Metrics) RecordLLMCall(model string, duration time.Duration, tokensIn, tokensOut int64, err error) {
	m.latencies.Record("llm:"+model, duration)
	m.TokensIn.Add(tokensIn)
	m.TokensOut.Add(tokensOut)
	if err != nil {
		m.errors.Record(fmt.Errorf("llm:%s: %w", model, err))
	}
}

// Snapshot returns a point-in-time copy of all metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	snap := MetricsSnapshot{
		Uptime:          time.Since(m.startTime),
		RequestsTotal:   m.RequestsTotal.Load(),
		RequestsSuccess: m.RequestsSuccess.Load(),
		RequestsFailed:  m.RequestsFailed.Load(),
		TokensIn:        m.TokensIn.Load(),
		TokensOut:       m.TokensOut.Load(),
		TokensTotal:     m.TokensIn.Load() + m.TokensOut.Load(),
		RequestLatency:  m.latencies.Stats("request"),
		Skills:          m.skills.Snapshot(),
		RecentErrors:    m.errors.Recent(20),
		LLMLatencies:    m.latencies.AllByPrefix("llm:"),
		BreakerTrips:    m.BreakerTrips.Load(),
		BreakerState:    "closed",
		Channels:        m.channels.Snapshot(),
		Knowledge:       m.knowledge.Snapshot(),
	}
	if m.breakerStatus != nil {
		snap.BreakerState, snap.BreakerFailures = m.breakerStatus()
	}
	return snap
}

// PrometheusFormat returns metrics in Prometheus text exposition format.
func (m *Metrics) PrometheusFormat() string {
	snap := m.Snapshot()
	var b strings.Builder

	w := func(name, help, typ string, value any) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n%s %v\n", name, help, name, typ, name, value)
	}

	w("yunque_requests_total", "Total requests", "counter", snap.RequestsTotal)
	w("yunque_requests_success", "Successful requests", "counter", snap.RequestsSuccess)
	w("yunque_requests_failed", "Failed requests", "counter", snap.RequestsFailed)
	w("yunque_tokens_in", "Input tokens consumed", "counter", snap.TokensIn)
	w("yunque_tokens_out", "Output tokens generated", "counter", snap.TokensOut)
	w("yunque_uptime_seconds", "Agent uptime", "gauge", int(snap.Uptime.Seconds()))

	if snap.RequestLatency.Count > 0 {
		fmt.Fprintf(&b, "# HELP yunque_request_duration_ms Request latency\n# TYPE yunque_request_duration_ms summary\n")
		fmt.Fprintf(&b, "yunque_request_duration_ms{quantile=\"0.5\"} %.1f\n", snap.RequestLatency.P50)
		fmt.Fprintf(&b, "yunque_request_duration_ms{quantile=\"0.95\"} %.1f\n", snap.RequestLatency.P95)
		fmt.Fprintf(&b, "yunque_request_duration_ms{quantile=\"0.99\"} %.1f\n", snap.RequestLatency.P99)
	}

	for _, sk := range snap.Skills {
		fmt.Fprintf(&b, "yunque_skill_calls_total{skill=%q} %d\n", sk.Name, sk.Total)
		fmt.Fprintf(&b, "yunque_skill_calls_success{skill=%q} %d\n", sk.Name, sk.Success)
		fmt.Fprintf(&b, "yunque_skill_calls_failed{skill=%q} %d\n", sk.Name, sk.Failed)
		fmt.Fprintf(&b, "yunque_skill_duration_ms{skill=%q,quantile=\"0.5\"} %.1f\n", sk.Name, sk.Latency.P50)
	}

	// Circuit breaker metrics
	w("yunque_breaker_trips_total", "Circuit breaker trip count", "counter", snap.BreakerTrips)
	fmt.Fprintf(&b, "# HELP yunque_breaker_state LLM circuit breaker state (0=closed,1=open,2=half-open)\n")
	fmt.Fprintf(&b, "# TYPE yunque_breaker_state gauge\n")
	breakerVal := 0
	switch snap.BreakerState {
	case "open":
		breakerVal = 1
	case "half-open":
		breakerVal = 2
	}
	fmt.Fprintf(&b, "yunque_breaker_state %d\n", breakerVal)

	// Channel metrics
	if len(snap.Channels) > 0 {
		fmt.Fprintf(&b, "# HELP yunque_channel_messages_in Incoming messages per channel\n# TYPE yunque_channel_messages_in counter\n")
		fmt.Fprintf(&b, "# HELP yunque_channel_send_total Outgoing messages per channel\n# TYPE yunque_channel_send_total counter\n")
		fmt.Fprintf(&b, "# HELP yunque_channel_send_failed Failed sends per channel\n# TYPE yunque_channel_send_failed counter\n")
		for _, ch := range snap.Channels {
			fmt.Fprintf(&b, "yunque_channel_messages_in{channel=%q} %d\n", ch.Channel, ch.MessagesIn)
			fmt.Fprintf(&b, "yunque_channel_send_total{channel=%q} %d\n", ch.Channel, ch.SendTotal)
			fmt.Fprintf(&b, "yunque_channel_send_failed{channel=%q} %d\n", ch.Channel, ch.SendFailed)
		}
	}

	// Knowledge search metrics
	if len(snap.Knowledge.Searches) > 0 {
		fmt.Fprintf(&b, "# HELP yunque_knowledge_search_total Knowledge searches per type\n# TYPE yunque_knowledge_search_total counter\n")
		for typ, count := range snap.Knowledge.Searches {
			fmt.Fprintf(&b, "yunque_knowledge_search_total{type=%q} %d\n", typ, count)
		}
		w("yunque_knowledge_results_total", "Total search results returned", "counter", snap.Knowledge.TotalResults)
		// Search latency percentiles
		for typ, lat := range snap.Knowledge.SearchLatency {
			if lat.Count > 0 {
				fmt.Fprintf(&b, "yunque_knowledge_search_duration_ms{type=%q,quantile=\"0.5\"} %.1f\n", typ, lat.P50)
				fmt.Fprintf(&b, "yunque_knowledge_search_duration_ms{type=%q,quantile=\"0.95\"} %.1f\n", typ, lat.P95)
				fmt.Fprintf(&b, "yunque_knowledge_search_duration_ms{type=%q,quantile=\"0.99\"} %.1f\n", typ, lat.P99)
			}
		}
	}

	// Rerank metrics
	if len(snap.Knowledge.RerankTotal) > 0 {
		fmt.Fprintf(&b, "# HELP yunque_rerank_total Rerank operations per provider\n# TYPE yunque_rerank_total counter\n")
		fmt.Fprintf(&b, "# HELP yunque_rerank_failed Failed rerank operations per provider\n# TYPE yunque_rerank_failed counter\n")
		for prov, count := range snap.Knowledge.RerankTotal {
			fmt.Fprintf(&b, "yunque_rerank_total{provider=%q} %d\n", prov, count)
			fmt.Fprintf(&b, "yunque_rerank_failed{provider=%q} %d\n", prov, snap.Knowledge.RerankFailed[prov])
		}
		// Rerank latency percentiles
		for prov, lat := range snap.Knowledge.RerankLatency {
			if lat.Count > 0 {
				fmt.Fprintf(&b, "yunque_rerank_duration_ms{provider=%q,quantile=\"0.5\"} %.1f\n", prov, lat.P50)
				fmt.Fprintf(&b, "yunque_rerank_duration_ms{provider=%q,quantile=\"0.95\"} %.1f\n", prov, lat.P95)
				fmt.Fprintf(&b, "yunque_rerank_duration_ms{provider=%q,quantile=\"0.99\"} %.1f\n", prov, lat.P99)
			}
		}
	}

	return b.String()
}

// --- Snapshot types ---

type MetricsSnapshot struct {
	Uptime          time.Duration           `json:"uptime"`
	RequestsTotal   int64                   `json:"requests_total"`
	RequestsSuccess int64                   `json:"requests_success"`
	RequestsFailed  int64                   `json:"requests_failed"`
	TokensIn        int64                   `json:"tokens_in"`
	TokensOut       int64                   `json:"tokens_out"`
	TokensTotal     int64                   `json:"tokens_total"`
	RequestLatency  LatencyStats            `json:"request_latency"`
	Skills          []SkillSnapshot         `json:"skills"`
	RecentErrors    []ErrorEntry            `json:"recent_errors"`
	LLMLatencies    map[string]LatencyStats `json:"llm_latencies"`
	BreakerState    string                  `json:"breaker_state"`
	BreakerTrips    int64                   `json:"breaker_trips"`
	BreakerFailures int                     `json:"breaker_failures"`
	Channels        []ChannelSnapshot       `json:"channels"`
	Knowledge       KnowledgeSnapshot       `json:"knowledge"`
}

type LatencyStats struct {
	Count int     `json:"count"`
	Avg   float64 `json:"avg_ms"`
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	P99   float64 `json:"p99_ms"`
	Max   float64 `json:"max_ms"`
}

type SkillSnapshot struct {
	Name        string       `json:"name"`
	Total       int64        `json:"total"`
	Success     int64        `json:"success"`
	Failed      int64        `json:"failed"`
	SuccessRate float64      `json:"success_rate"`
	Latency     LatencyStats `json:"latency"`
}

type ErrorEntry struct {
	Message string    `json:"message"`
	Count   int       `json:"count"`
	Last    time.Time `json:"last"`
}

type ChannelSnapshot struct {
	Channel    string `json:"channel"`
	MessagesIn int64  `json:"messages_in"`
	SendTotal  int64  `json:"send_total"`
	SendFailed int64  `json:"send_failed"`
}

type KnowledgeSnapshot struct {
	Searches      map[string]int64        `json:"searches"`
	SearchLatency map[string]LatencyStats `json:"search_latency"`
	TotalResults  int64                   `json:"total_results"`
	RerankTotal   map[string]int64        `json:"rerank_total"`
	RerankFailed  map[string]int64        `json:"rerank_failed"`
	RerankLatency map[string]LatencyStats `json:"rerank_latency"`
}

// --- Histogram ---

type HistogramStore struct {
	mu      sync.RWMutex
	buckets map[string][]float64 // name -> latencies in ms
	maxSize int
}

func newHistogramStore() *HistogramStore {
	return &HistogramStore{buckets: make(map[string][]float64), maxSize: 10000}
}

func (h *HistogramStore) Record(name string, d time.Duration) {
	ms := float64(d.Microseconds()) / 1000.0
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buckets[name] = append(h.buckets[name], ms)
	// Evict oldest if over max
	if len(h.buckets[name]) > h.maxSize {
		h.buckets[name] = h.buckets[name][len(h.buckets[name])-h.maxSize:]
	}
}

func (h *HistogramStore) Stats(name string) LatencyStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	vals := h.buckets[name]
	if len(vals) == 0 {
		return LatencyStats{}
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)

	sum := 0.0
	for _, v := range sorted {
		sum += v
	}

	return LatencyStats{
		Count: len(sorted),
		Avg:   math.Round(sum/float64(len(sorted))*10) / 10,
		P50:   percentile(sorted, 0.50),
		P95:   percentile(sorted, 0.95),
		P99:   percentile(sorted, 0.99),
		Max:   sorted[len(sorted)-1],
	}
}

func (h *HistogramStore) AllByPrefix(prefix string) map[string]LatencyStats {
	h.mu.RLock()
	names := make([]string, 0)
	for name := range h.buckets {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	h.mu.RUnlock()

	out := make(map[string]LatencyStats)
	for _, name := range names {
		out[strings.TrimPrefix(name, prefix)] = h.Stats(name)
	}
	return out
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return math.Round(sorted[idx]*10) / 10
}

// --- Skill metrics ---

type SkillMetrics struct {
	mu    sync.RWMutex
	calls map[string]*skillCounter
}

type skillCounter struct {
	total   atomic.Int64
	success atomic.Int64
	failed  atomic.Int64
	hist    *HistogramStore
}

func newSkillMetrics() *SkillMetrics {
	return &SkillMetrics{calls: make(map[string]*skillCounter)}
}

func (s *SkillMetrics) Record(name string, d time.Duration, err error) {
	s.mu.RLock()
	sc, ok := s.calls[name]
	s.mu.RUnlock()

	if !ok {
		s.mu.Lock()
		sc, ok = s.calls[name]
		if !ok {
			sc = &skillCounter{hist: newHistogramStore()}
			s.calls[name] = sc
		}
		s.mu.Unlock()
	}

	sc.total.Add(1)
	if err != nil {
		sc.failed.Add(1)
	} else {
		sc.success.Add(1)
	}
	sc.hist.Record(name, d)
}

func (s *SkillMetrics) Snapshot() []SkillSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]SkillSnapshot, 0, len(s.calls))
	for name, sc := range s.calls {
		total := sc.total.Load()
		success := sc.success.Load()
		rate := 0.0
		if total > 0 {
			rate = float64(success) / float64(total)
		}
		out = append(out, SkillSnapshot{
			Name:        name,
			Total:       total,
			Success:     success,
			Failed:      sc.failed.Load(),
			SuccessRate: math.Round(rate*1000) / 1000,
			Latency:     sc.hist.Stats(name),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out
}

// --- Error tracker ---

type ErrorTracker struct {
	mu     sync.Mutex
	errors map[string]*errorRecord
}

type errorRecord struct {
	message string
	count   int
	last    time.Time
}

func newErrorTracker() *ErrorTracker {
	return &ErrorTracker{errors: make(map[string]*errorRecord)}
}

func (e *ErrorTracker) Record(err error) {
	msg := err.Error()
	// Truncate long messages
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if rec, ok := e.errors[msg]; ok {
		rec.count++
		rec.last = time.Now()
	} else {
		e.errors[msg] = &errorRecord{message: msg, count: 1, last: time.Now()}
	}
}

func (e *ErrorTracker) Recent(n int) []ErrorEntry {
	e.mu.Lock()
	defer e.mu.Unlock()

	entries := make([]ErrorEntry, 0, len(e.errors))
	for _, rec := range e.errors {
		entries = append(entries, ErrorEntry{Message: rec.message, Count: rec.count, Last: rec.last})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Last.After(entries[j].Last) })
	if len(entries) > n {
		entries = entries[:n]
	}
	return entries
}

// --- Channel metrics ---

type channelCounter struct {
	in     atomic.Int64
	out    atomic.Int64
	failed atomic.Int64
}

// ChannelMetrics tracks per-channel message counters.
type ChannelMetrics struct {
	mu       sync.RWMutex
	channels map[string]*channelCounter
}

func newChannelMetrics() *ChannelMetrics {
	return &ChannelMetrics{channels: make(map[string]*channelCounter)}
}

func (c *ChannelMetrics) get(ch string) *channelCounter {
	c.mu.RLock()
	cc, ok := c.channels[ch]
	c.mu.RUnlock()
	if ok {
		return cc
	}
	c.mu.Lock()
	cc, ok = c.channels[ch]
	if !ok {
		cc = &channelCounter{}
		c.channels[ch] = cc
	}
	c.mu.Unlock()
	return cc
}

func (c *ChannelMetrics) RecordIn(channelType string) {
	c.get(channelType).in.Add(1)
}

func (c *ChannelMetrics) RecordOut(channelType string, err error) {
	cc := c.get(channelType)
	cc.out.Add(1)
	if err != nil {
		cc.failed.Add(1)
	}
}

func (c *ChannelMetrics) Snapshot() []ChannelSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ChannelSnapshot, 0, len(c.channels))
	for name, cc := range c.channels {
		out = append(out, ChannelSnapshot{
			Channel:    name,
			MessagesIn: cc.in.Load(),
			SendTotal:  cc.out.Load(),
			SendFailed: cc.failed.Load(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MessagesIn > out[j].MessagesIn })
	return out
}

// --- Knowledge metrics ---

type searchCounter struct {
	total   atomic.Int64
	results atomic.Int64
}

type rerankCounter struct {
	total  atomic.Int64
	failed atomic.Int64
}

// KnowledgeMetrics tracks RAG search and rerank operations.
type KnowledgeMetrics struct {
	mu       sync.RWMutex
	searches map[string]*searchCounter
	reranks  map[string]*rerankCounter
	hist     *HistogramStore
}

func newKnowledgeMetrics() *KnowledgeMetrics {
	return &KnowledgeMetrics{
		searches: make(map[string]*searchCounter),
		reranks:  make(map[string]*rerankCounter),
		hist:     newHistogramStore(),
	}
}

func (k *KnowledgeMetrics) getSearch(typ string) *searchCounter {
	k.mu.RLock()
	sc, ok := k.searches[typ]
	k.mu.RUnlock()
	if ok {
		return sc
	}
	k.mu.Lock()
	sc, ok = k.searches[typ]
	if !ok {
		sc = &searchCounter{}
		k.searches[typ] = sc
	}
	k.mu.Unlock()
	return sc
}

func (k *KnowledgeMetrics) getRerank(provider string) *rerankCounter {
	k.mu.RLock()
	rc, ok := k.reranks[provider]
	k.mu.RUnlock()
	if ok {
		return rc
	}
	k.mu.Lock()
	rc, ok = k.reranks[provider]
	if !ok {
		rc = &rerankCounter{}
		k.reranks[provider] = rc
	}
	k.mu.Unlock()
	return rc
}

func (k *KnowledgeMetrics) RecordSearch(searchType string, duration time.Duration, results int) {
	sc := k.getSearch(searchType)
	sc.total.Add(1)
	sc.results.Add(int64(results))
	k.hist.Record("search:"+searchType, duration)
}

func (k *KnowledgeMetrics) RecordRerank(provider string, duration time.Duration, err error) {
	rc := k.getRerank(provider)
	rc.total.Add(1)
	if err != nil {
		rc.failed.Add(1)
	}
	k.hist.Record("rerank:"+provider, duration)
}

func (k *KnowledgeMetrics) Snapshot() KnowledgeSnapshot {
	k.mu.RLock()
	defer k.mu.RUnlock()

	searches := make(map[string]int64, len(k.searches))
	searchLat := make(map[string]LatencyStats, len(k.searches))
	var totalResults int64
	for typ, sc := range k.searches {
		searches[typ] = sc.total.Load()
		totalResults += sc.results.Load()
		searchLat[typ] = k.hist.Stats("search:" + typ)
	}

	rerankTotal := make(map[string]int64, len(k.reranks))
	rerankFailed := make(map[string]int64, len(k.reranks))
	rerankLat := make(map[string]LatencyStats, len(k.reranks))
	for prov, rc := range k.reranks {
		rerankTotal[prov] = rc.total.Load()
		rerankFailed[prov] = rc.failed.Load()
		rerankLat[prov] = k.hist.Stats("rerank:" + prov)
	}

	return KnowledgeSnapshot{
		Searches:      searches,
		SearchLatency: searchLat,
		TotalResults:  totalResults,
		RerankTotal:   rerankTotal,
		RerankFailed:  rerankFailed,
		RerankLatency: rerankLat,
	}
}
