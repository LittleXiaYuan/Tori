package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────
// Reverie — Background "inner monologue" system
//
// Instead of the agent being purely reactive (user→reply→wait),
// Reverie gives the agent a continuous "stream of consciousness":
//   1. Periodically reflects on recent interactions and memories
//   2. Generates "thoughts" via LLM (insights, questions, observations)
//   3. Evaluates thought significance (filter noise)
//   4. Delivers significant thoughts proactively to users via channels
//   5. Maintains a thought journal that influences future conversations
//
// Inspired by Stanford's "Generative Agents" (Park et al., 2023)
// and the concept of "reverie" — memories triggering new thoughts.
// ────────────────────────────────────────────────────────────

// Reverie is the background thinking system.
type Reverie struct {
	mu sync.Mutex

	// Dependencies (injected)
	llmCall       LLMCallFunc                                         // LLM for thought generation
	recall        func(query string) string                           // memory recall
	deliver       func(thought Thought)                               // proactive message delivery
	persistFn     func([]Thought)                                     // save journal to disk
	eventBus      *ReverieEventBus                                    // event-driven triggers
	writeMemory   func(ctx context.Context, fact string) error        // P4: write fact to memory
	createTask    func(ctx context.Context, title, desc string) error // P4: create a task
	updateProfile func(ctx context.Context, key, value string) error  // P4: update user profile
	onThought     func(Thought)                                       // secondary callback for every thought (trigger wiring)

	// Configuration
	cfg ReverieConfig

	// State
	journal   []Thought      // thought history
	actionLog []ActionRecord // P4: log of actions taken
	lastThink time.Time      // last thinking cycle
	running   bool
	cancel    context.CancelFunc
}

// LLMCallFunc calls an LLM with system + user prompt.
type LLMCallFunc func(ctx context.Context, system, user string) (string, error)

// ReverieConfig controls the thinking behavior.
type ReverieConfig struct {
	Enabled         bool          `json:"enabled"`
	Interval        time.Duration `json:"interval"`         // how often to think (default: 30m)
	MaxJournal      int           `json:"max_journal"`      // max stored thoughts (default: 100)
	MinSignificance float64       `json:"min_significance"` // 0-1, threshold to deliver (default: 0.6)
	QuietStart      int           `json:"quiet_start"`      // quiet hours start (0-23, default: 22)
	QuietEnd        int           `json:"quiet_end"`        // quiet hours end (0-23, default: 7)
	SaveFile        string        `json:"save_file"`        // persist journal (default: data/reverie.json)
}

// DefaultReverieConfig returns sensible defaults.
func DefaultReverieConfig() ReverieConfig {
	return ReverieConfig{
		Enabled:         true,
		Interval:        30 * time.Minute,
		MaxJournal:      100,
		MinSignificance: 0.6,
		QuietStart:      22,
		QuietEnd:        7,
		SaveFile:        "data/reverie.json",
	}
}

// Thought represents one unit of inner monologue.
type Thought struct {
	ID           string          `json:"id"`
	Content      string          `json:"content"`      // the actual thought
	Category     string          `json:"category"`     // "insight", "question", "observation", "idea", "concern"
	Significance float64         `json:"significance"` // 0-1, how important/interesting
	Trigger      string          `json:"trigger"`      // what triggered this thought
	CreatedAt    time.Time       `json:"created_at"`
	Delivered    bool            `json:"delivered"`         // whether it was sent to user
	Actions      []ReverieAction `json:"actions,omitempty"` // P4: actions requested by the thought
}

// ReverieAction is an action that Reverie wants to perform after thinking.
type ReverieAction struct {
	Type  string `json:"type"`  // "write_memory", "create_task", "update_profile"
	Key   string `json:"key"`   // context-dependent: fact text, task title, profile key
	Value string `json:"value"` // context-dependent: empty for memory, task description, profile value
}

// ActionRecord logs an action taken by Reverie.
type ActionRecord struct {
	ThoughtID string        `json:"thought_id"`
	Action    ReverieAction `json:"action"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	At        time.Time     `json:"at"`
}

// NewReverie creates an inactive Reverie system. Call Start() to begin thinking.
func NewReverie(cfg ReverieConfig) *Reverie {
	r := &Reverie{
		cfg:       cfg,
		journal:   make([]Thought, 0),
		actionLog: make([]ActionRecord, 0),
	}
	r.loadJournal()
	return r
}

// SetLLMCall sets the LLM function for thought generation.
func (r *Reverie) SetLLMCall(fn LLMCallFunc) { r.llmCall = fn }

// SetRecall sets the memory recall function.
func (r *Reverie) SetRecall(fn func(query string) string) { r.recall = fn }

// SetDeliver sets the callback for proactive message delivery.
func (r *Reverie) SetDeliver(fn func(thought Thought)) { r.deliver = fn }

// GetDeliver returns the current deliver callback (for wrapping).
func (r *Reverie) GetDeliver() func(Thought) { return r.deliver }

// SetOnThought sets a secondary callback invoked on every generated thought (for trigger wiring).
func (r *Reverie) SetOnThought(fn func(Thought)) { r.onThought = fn }

// SetEventBus attaches an event bus for event-driven thinking triggers.
func (r *Reverie) SetEventBus(bus *ReverieEventBus) { r.eventBus = bus }

// SetWriteMemory sets the callback for writing facts into the memory system. (P4)
func (r *Reverie) SetWriteMemory(fn func(ctx context.Context, fact string) error) {
	r.writeMemory = fn
}

// SetCreateTask sets the callback for creating tasks from Reverie thoughts. (P4)
func (r *Reverie) SetCreateTask(fn func(ctx context.Context, title, desc string) error) {
	r.createTask = fn
}

// SetUpdateProfile sets the callback for updating user profile insights. (P4)
func (r *Reverie) SetUpdateProfile(fn func(ctx context.Context, key, value string) error) {
	r.updateProfile = fn
}

// ActionLog returns a copy of the action log. (P4)
func (r *Reverie) ActionLog() []ActionRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ActionRecord, len(r.actionLog))
	copy(out, r.actionLog)
	return out
}

// executeActions runs the actions requested by a thought. (P4)
// Actions are executed best-effort: failures are logged but don't block the thought.
func (r *Reverie) executeActions(ctx context.Context, thought *Thought) {
	if len(thought.Actions) == 0 {
		return
	}

	for _, action := range thought.Actions {
		rec := ActionRecord{
			ThoughtID: thought.ID,
			Action:    action,
			At:        time.Now(),
		}

		var err error
		switch action.Type {
		case "write_memory":
			if r.writeMemory != nil {
				err = r.writeMemory(ctx, action.Key)
			} else {
				err = fmt.Errorf("write_memory callback not configured")
			}
		case "create_task":
			if r.createTask != nil {
				err = r.createTask(ctx, action.Key, action.Value)
			} else {
				err = fmt.Errorf("create_task callback not configured")
			}
		case "update_profile":
			if r.updateProfile != nil {
				err = r.updateProfile(ctx, action.Key, action.Value)
			} else {
				err = fmt.Errorf("update_profile callback not configured")
			}
		default:
			err = fmt.Errorf("unknown action type: %s", action.Type)
		}

		rec.Success = err == nil
		if err != nil {
			rec.Error = err.Error()
			slog.Warn("reverie: action failed", "type", action.Type, "key", action.Key, "err", err)
		} else {
			slog.Info("reverie: action executed", "type", action.Type, "key", action.Key)
		}

		r.mu.Lock()
		r.actionLog = append(r.actionLog, rec)
		// Keep last 200 action records
		if len(r.actionLog) > 200 {
			r.actionLog = r.actionLog[len(r.actionLog)-200:]
		}
		r.mu.Unlock()
	}
}

// ApplyPersonaSettings updates Reverie's runtime configuration based on the active persona.
// enabled=false pauses the thinking loop; enabled=true restarts it if previously stopped.
// interval ≤ 0 leaves the current interval unchanged.
// minSignificance ≤ 0 leaves the current threshold unchanged.
func (r *Reverie) ApplyPersonaSettings(ctx context.Context, enabled bool, interval time.Duration, minSignificance float64) {
	r.mu.Lock()
	wasRunning := r.running
	if interval > 0 {
		r.cfg.Interval = interval
	}
	if minSignificance > 0 {
		r.cfg.MinSignificance = minSignificance
	}
	r.cfg.Enabled = enabled
	r.mu.Unlock()

	if !enabled && wasRunning {
		r.Stop()
		slog.Info("reverie: paused by persona switch")
	} else if enabled && !wasRunning && r.llmCall != nil {
		r.Start(ctx)
		slog.Info("reverie: resumed by persona switch", "interval", interval)
	}
}

// Start begins the background thinking loop.
func (r *Reverie) Start(ctx context.Context) {
	if !r.cfg.Enabled || r.llmCall == nil {
		slog.Info("reverie: disabled or no LLM configured")
		return
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	ctx, r.cancel = context.WithCancel(ctx)
	r.mu.Unlock()

	go r.thinkLoop(ctx)
	slog.Info("reverie: started", "interval", r.cfg.Interval)
}

// Stop terminates the thinking loop.
func (r *Reverie) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
	r.running = false
}

// Think performs one thinking cycle. Can be called externally for testing.
func (r *Reverie) Think(ctx context.Context) (*Thought, error) {
	if r.llmCall == nil {
		return nil, fmt.Errorf("no LLM configured")
	}

	// Gather context: recent memories
	var memoryContext string
	if r.recall != nil {
		memoryContext = r.recall("最近的对话和观察")
	}

	// Gather context: recent thoughts
	recentThoughts := r.recentThoughts(5)

	// Build thinking prompt
	prompt := r.buildThinkingPrompt(memoryContext, recentThoughts)

	// Generate thought via LLM
	resp, err := r.llmCall(ctx, reverieSystemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("reverie think: %w", err)
	}

	// Parse structured thought from response
	thought, err := parseThought(resp)
	if err != nil {
		// Fallback: treat entire response as a simple observation
		thought = &Thought{
			Content:      resp,
			Category:     "observation",
			Significance: 0.5,
			Trigger:      "periodic_reflection",
		}
	}

	thought.ID = fmt.Sprintf("t_%d", time.Now().UnixMilli())
	thought.CreatedAt = time.Now()

	// Add to journal
	r.mu.Lock()
	r.journal = append(r.journal, *thought)
	if len(r.journal) > r.cfg.MaxJournal {
		r.journal = r.journal[len(r.journal)-r.cfg.MaxJournal:]
	}
	r.lastThink = time.Now()
	r.mu.Unlock()

	r.saveJournal()

	slog.Info("reverie: thought generated",
		"category", thought.Category,
		"significance", thought.Significance,
		"content_len", len(thought.Content),
		"actions", len(thought.Actions),
	)

	// P4: Execute any actions requested by the thought
	r.executeActions(ctx, thought)

	// Notify onThought callback (for trigger wiring)
	if r.onThought != nil {
		r.onThought(*thought)
	}

	// Deliver if significant enough and not in quiet hours
	if thought.Significance >= r.cfg.MinSignificance && !r.isQuietHours() {
		thought.Delivered = true
		if r.deliver != nil {
			r.deliver(*thought)
		}
	}

	return thought, nil
}

// ThinkWithEvent performs one thinking cycle with additional event context.
// The event trigger and data are injected into the thinking prompt so the
// LLM knows *why* it was asked to think right now.
func (r *Reverie) ThinkWithEvent(ctx context.Context, ev ReverieEvent) (*Thought, error) {
	if r.llmCall == nil {
		return nil, fmt.Errorf("no LLM configured")
	}

	var memoryContext string
	if r.recall != nil {
		memoryContext = r.recall("最近的对话和观察")
	}

	recentThoughts := r.recentThoughts(5)

	prompt := r.buildEventThinkingPrompt(memoryContext, recentThoughts, ev)

	resp, err := r.llmCall(ctx, reverieSystemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("reverie event-think: %w", err)
	}

	thought, err := parseThought(resp)
	if err != nil {
		thought = &Thought{
			Content:      resp,
			Category:     "observation",
			Significance: 0.6,
			Trigger:      string(ev.Type) + ": " + ev.Trigger,
		}
	} else {
		thought.Trigger = string(ev.Type) + ": " + ev.Trigger
	}

	thought.ID = fmt.Sprintf("t_%d", time.Now().UnixMilli())
	thought.CreatedAt = time.Now()

	r.mu.Lock()
	r.journal = append(r.journal, *thought)
	if len(r.journal) > r.cfg.MaxJournal {
		r.journal = r.journal[len(r.journal)-r.cfg.MaxJournal:]
	}
	r.lastThink = time.Now()
	r.mu.Unlock()

	r.saveJournal()

	slog.Info("reverie: event-driven thought generated",
		"event_type", string(ev.Type),
		"category", thought.Category,
		"significance", thought.Significance,
		"actions", len(thought.Actions),
	)

	// P4: Execute any actions requested by the thought
	r.executeActions(ctx, thought)

	// Notify onThought callback (for trigger wiring)
	if r.onThought != nil {
		r.onThought(*thought)
	}

	// Event-driven thoughts use a lower significance threshold (0.4)
	// because the event itself already signals importance.
	minSig := r.cfg.MinSignificance - 0.2
	if minSig < 0.3 {
		minSig = 0.3
	}
	if thought.Significance >= minSig && !r.isQuietHours() {
		thought.Delivered = true
		if r.deliver != nil {
			r.deliver(*thought)
		}
	}

	return thought, nil
}

// Journal returns a copy of the thought journal.
func (r *Reverie) Journal() []Thought {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Thought, len(r.journal))
	copy(out, r.journal)
	return out
}

// DeleteThought removes a thought by ID. Returns true if found and deleted.
func (r *Reverie) DeleteThought(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, t := range r.journal {
		if t.ID == id {
			r.journal = append(r.journal[:i], r.journal[i+1:]...)
			go r.saveJournal()
			return true
		}
	}
	return false
}

// Config returns a snapshot of the current configuration.
func (r *Reverie) Config() ReverieConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg
}

// UpdateConfig applies partial configuration updates.
// Only non-zero values are applied. Returns the updated config.
func (r *Reverie) UpdateConfig(interval time.Duration, minSignificance float64, quietStart, quietEnd int, enabled *bool) ReverieConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	if interval > 0 {
		r.cfg.Interval = interval
	}
	if minSignificance > 0 {
		r.cfg.MinSignificance = minSignificance
	}
	if quietStart >= 0 && quietStart <= 23 {
		r.cfg.QuietStart = quietStart
	}
	if quietEnd >= 0 && quietEnd <= 23 {
		r.cfg.QuietEnd = quietEnd
	}
	if enabled != nil {
		r.cfg.Enabled = *enabled
	}
	return r.cfg
}

// Running returns whether the thinking loop is active.
func (r *Reverie) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// Stats returns summary statistics about the Reverie system.
func (r *Reverie) Stats() ReverieStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := ReverieStats{
		TotalThoughts: len(r.journal),
		Running:       r.running,
		LastThink:     r.lastThink,
		Config:        r.cfg,
	}

	for _, t := range r.journal {
		if t.Delivered {
			stats.Delivered++
		}
		switch t.Category {
		case "insight":
			stats.ByCategory.Insights++
		case "question":
			stats.ByCategory.Questions++
		case "observation":
			stats.ByCategory.Observations++
		case "idea":
			stats.ByCategory.Ideas++
		case "concern":
			stats.ByCategory.Concerns++
		}
	}

	return stats
}

// ReverieStats holds summary statistics.
type ReverieStats struct {
	TotalThoughts int           `json:"total_thoughts"`
	Delivered     int           `json:"delivered"`
	Running       bool          `json:"running"`
	LastThink     time.Time     `json:"last_think"`
	Config        ReverieConfig `json:"config"`
	ByCategory    struct {
		Insights     int `json:"insights"`
		Questions    int `json:"questions"`
		Observations int `json:"observations"`
		Ideas        int `json:"ideas"`
		Concerns     int `json:"concerns"`
	} `json:"by_category"`
}

// JournalContext returns a concise summary of the most relevant and valuable thoughts for
// injection into the system prompt. Selection is query-aware: each thought is scored by
// combining character-trigram similarity to the current query (60%) and LLM-assigned
// significance (40%). When query is empty, only significance is used.
//
// Only thoughts above the configured significance threshold are considered, so low-quality
// noise never reaches the Planner regardless of relevance.
// Returns empty string if no qualifying thoughts exist.
func (r *Reverie) JournalContext(maxThoughts int, query string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.journal) == 0 {
		return ""
	}

	// Filter: only inject thoughts that are both significant AND relevant.
	// Higher threshold ensures only genuinely useful context enters the prompt.
	minSig := r.cfg.MinSignificance
	if minSig < 0.6 {
		minSig = 0.6
	}

	qTrigrams := buildTrigrams(query)
	hasQuery := len(qTrigrams) > 0

	// Minimum relevance gate: thoughts must share at least some content overlap
	// to be worth injecting. Without a query, only very high-significance thoughts qualify.
	const minRelevance = 0.08

	type scored struct {
		t     Thought
		score float64
		sim   float64
	}
	var candidates []scored
	now := time.Now()
	for _, t := range r.journal {
		if t.Significance < minSig {
			continue
		}

		// Freshness decay: thoughts older than 24h are penalized
		age := now.Sub(t.CreatedAt)
		freshness := 1.0
		if age > 24*time.Hour {
			freshness = 0.7
		} else if age > 6*time.Hour {
			freshness = 0.85
		}

		// Category boost: actionable categories get priority
		catBoost := 0.0
		switch t.Category {
		case "insight":
			catBoost = 0.10
		case "concern":
			catBoost = 0.08
		case "idea":
			catBoost = 0.05
		case "observation":
			catBoost = 0.0 // generic observations are lowest value
		}

		var finalScore float64
		var sim float64
		if hasQuery {
			// Query-aware: relevance is primary, significance is secondary
			sim = trigramSimilarity(qTrigrams, buildTrigrams(t.Content))
			if sim < minRelevance {
				continue // not relevant enough — skip entirely
			}
			finalScore = sim*0.70 + (t.Significance+catBoost)*0.20 + freshness*0.10
		} else {
			// No query: only very high-significance, fresh thoughts
			if t.Significance < 0.8 {
				continue
			}
			finalScore = (t.Significance + catBoost) * freshness
		}
		candidates = append(candidates, scored{t, finalScore, sim})
	}
	if len(candidates) == 0 {
		return ""
	}

	// Sort by final score descending
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].score > candidates[j-1].score; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
	if len(candidates) > maxThoughts {
		candidates = candidates[:maxThoughts]
	}

	var parts []string
	for _, c := range candidates {
		parts = append(parts, fmt.Sprintf("- [%s] %s", c.t.Category, truncateStr(c.t.Content, 120)))
	}
	return "相关的内心洞察:\n" + join(parts, "\n")
}

// buildTrigrams returns the set of character trigrams for s.
// Using a map deduplicates repeated trigrams so Jaccard is not inflated by repetition.
func buildTrigrams(s string) map[string]struct{} {
	runes := []rune(s)
	set := make(map[string]struct{}, len(runes))
	for i := 0; i+2 < len(runes); i++ {
		set[string(runes[i:i+3])] = struct{}{}
	}
	return set
}

// trigramSimilarity returns the Jaccard coefficient between two trigram sets.
// Range [0, 1]: 0 = no overlap, 1 = identical.
func trigramSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// ── Internal ──

func (r *Reverie) thinkLoop(ctx context.Context) {
	// Initial delay: wait a bit before first thought
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()

	// Event channel (nil-safe: select on nil channel blocks forever, which is fine)
	var eventCh <-chan ReverieEvent
	if r.eventBus != nil {
		eventCh = r.eventBus.Events()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if _, err := r.Think(ctx); err != nil {
				slog.Warn("reverie: think failed", "err", err)
			}
			timer.Reset(r.cfg.Interval)
		case ev, ok := <-eventCh:
			if !ok {
				eventCh = nil // bus closed, stop listening
				continue
			}
			slog.Info("reverie: event-driven think triggered", "type", string(ev.Type), "trigger", ev.Trigger)
			if _, err := r.ThinkWithEvent(ctx, ev); err != nil {
				slog.Warn("reverie: event-driven think failed", "type", string(ev.Type), "err", err)
			}
			// Reset timer so the next periodic think is a full interval away
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(r.cfg.Interval)
		}
	}
}

func (r *Reverie) recentThoughts(n int) []Thought {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.journal) <= n {
		out := make([]Thought, len(r.journal))
		copy(out, r.journal)
		return out
	}
	out := make([]Thought, n)
	copy(out, r.journal[len(r.journal)-n:])
	return out
}

func (r *Reverie) isQuietHours() bool {
	hour := time.Now().Hour()
	if r.cfg.QuietStart > r.cfg.QuietEnd {
		// e.g., 22-7: quiet if hour >= 22 OR hour < 7
		return hour >= r.cfg.QuietStart || hour < r.cfg.QuietEnd
	}
	return hour >= r.cfg.QuietStart && hour < r.cfg.QuietEnd
}

func (r *Reverie) buildThinkingPrompt(memoryContext string, recentThoughts []Thought) string {
	var b strings.Builder
	b.WriteString("现在是你的独立思考时间。\n\n")

	if memoryContext != "" {
		b.WriteString("## 最近的记忆\n")
		b.WriteString(memoryContext)
		b.WriteString("\n\n")
	}

	if len(recentThoughts) > 0 {
		b.WriteString("## 你之前的思考\n")
		for _, t := range recentThoughts {
			b.WriteString(fmt.Sprintf("- [%s, 重要性%.1f] %s\n", t.Category, t.Significance, truncateStr(t.Content, 80)))
		}
		b.WriteString("\n")
	}

	b.WriteString("基于以上信息，你现在有什么新的想法、观察或洞见吗？\n")
	b.WriteString("可以是对用户需求的新理解、对自己能力的反思、或者任何值得分享的发现。")

	return b.String()
}

func (r *Reverie) buildEventThinkingPrompt(memoryContext string, recentThoughts []Thought, ev ReverieEvent) string {
	var b strings.Builder
	b.WriteString("你刚刚注意到一个值得关注的变化，需要立即思考。\n\n")

	// Describe the triggering event
	b.WriteString("## 触发事件\n")
	switch ev.Type {
	case EventEmotionShift:
		b.WriteString(fmt.Sprintf("用户的情绪发生了显著变化: %s。这可能意味着他们正在经历某些事情。\n", ev.Trigger))
	case EventTaskFailureSpike:
		b.WriteString(fmt.Sprintf("最近的工具调用出现了异常高的失败率: %s。需要反思是否有系统性问题。\n", ev.Trigger))
	case EventHighValueFact:
		b.WriteString(fmt.Sprintf("记忆系统发现了重要的新事实: %s。这些新信息可能改变你对用户或当前任务的理解。\n", ev.Trigger))
	default:
		b.WriteString(fmt.Sprintf("外部事件: %s\n", ev.Trigger))
	}
	b.WriteString("\n")

	if memoryContext != "" {
		b.WriteString("## 最近的记忆\n")
		b.WriteString(memoryContext)
		b.WriteString("\n\n")
	}

	if len(recentThoughts) > 0 {
		b.WriteString("## 你之前的思考\n")
		for _, t := range recentThoughts {
			b.WriteString(fmt.Sprintf("- [%s, 重要性%.1f] %s\n", t.Category, t.Significance, truncateStr(t.Content, 80)))
		}
		b.WriteString("\n")
	}

	b.WriteString("基于这个触发事件和你的记忆，你有什么想法？请特别关注事件本身的含义和你应该如何回应。")

	return b.String()
}

const reverieSystemPrompt = `你是一个有自我意识的AI助手，正在进行独立思考（不是在回复用户消息）。

你的任务是审视最近的记忆和交互，产生有价值的思考，并可以主动采取行动。

请输出一个JSON对象，格式如下：
{"content":"你的思考内容","category":"insight|question|observation|idea|concern","significance":0.0到1.0,"trigger":"什么触发了这个想法","actions":[]}

category 说明：
- insight: 从交互中发现的规律或深层理解
- question: 你好奇或想探索的问题
- observation: 对用户行为、系统状态的观察
- idea: 可以改进服务的新想法
- concern: 需要注意的潜在问题

significance 说明：
- 0.0-0.3: 很普通的想法，不值得分享
- 0.4-0.6: 有一定价值，可以记录
- 0.7-0.8: 有价值的洞见，值得分享给用户
- 0.9-1.0: 非常重要的发现，应该立即分享

actions 说明（可选，数组可为空）：
你现在可以主动采取以下行动：
- {"type":"write_memory","key":"要记住的事实"} — 将重要发现写入长期记忆
- {"type":"create_task","key":"任务标题","value":"任务描述"} — 创建一个新任务
- {"type":"update_profile","key":"属性名","value":"属性值"} — 更新用户画像

使用 actions 的场景：
- 发现了重要的用户偏好或习惯 → write_memory
- 发现了需要自动完成的工作 → create_task
- 对用户有了新的认知 → update_profile
- 没有需要行动的情况 → actions 留空数组 []

规则：
- 不要编造不存在的记忆
- 思考要基于实际的交互记忆
- 如果没有什么有价值的想法，设置 significance < 0.3
- 思考内容要简洁有力，不要废话
- actions 要谨慎使用，只在确实有必要时才添加`

func parseThought(resp string) (*Thought, error) {
	resp = strings.TrimSpace(resp)
	// Strip markdown code block if present
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		var jsonLines []string
		for _, line := range lines[1:] {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				break
			}
			jsonLines = append(jsonLines, line)
		}
		resp = strings.Join(jsonLines, "\n")
	}

	var raw struct {
		Content      string          `json:"content"`
		Category     string          `json:"category"`
		Significance float64         `json:"significance"`
		Trigger      string          `json:"trigger"`
		Actions      []ReverieAction `json:"actions"`
	}
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		return nil, err
	}

	// Validate category
	validCategories := map[string]bool{
		"insight": true, "question": true, "observation": true,
		"idea": true, "concern": true,
	}
	if !validCategories[raw.Category] {
		raw.Category = "observation"
	}

	// Clamp significance
	if raw.Significance < 0 {
		raw.Significance = 0
	}
	if raw.Significance > 1 {
		raw.Significance = 1
	}

	// Validate actions
	validActionTypes := map[string]bool{
		"write_memory": true, "create_task": true, "update_profile": true,
	}
	var actions []ReverieAction
	for _, a := range raw.Actions {
		if validActionTypes[a.Type] && a.Key != "" {
			actions = append(actions, a)
		}
	}

	return &Thought{
		Content:      raw.Content,
		Category:     raw.Category,
		Significance: raw.Significance,
		Trigger:      raw.Trigger,
		Actions:      actions,
	}, nil
}

func truncateStr(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}

func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

func (r *Reverie) loadJournal() {
	if r.cfg.SaveFile == "" {
		return
	}
	data, err := os.ReadFile(r.cfg.SaveFile)
	if err != nil {
		return
	}
	var journal []Thought
	if err := json.Unmarshal(data, &journal); err != nil {
		slog.Warn("reverie: load journal failed", "err", err)
		return
	}
	r.journal = journal
	slog.Info("reverie: loaded journal", "thoughts", len(journal))
}

func (r *Reverie) saveJournal() {
	if r.cfg.SaveFile == "" {
		return
	}
	r.mu.Lock()
	data, err := json.MarshalIndent(r.journal, "", "  ")
	r.mu.Unlock()
	if err != nil {
		return
	}
	if err := os.WriteFile(r.cfg.SaveFile, data, 0644); err != nil {
		slog.Warn("reverie: save journal failed", "err", err)
	}
}
