package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	iledger "yunque-agent/internal/ledger"
)

// Reverie — background "inner monologue" system.
//
// Instead of being purely reactive (user→reply→wait), Reverie gives the agent
// a continuous stream of consciousness: periodic reflection, thought generation,
// significance filtering, and proactive delivery.
//
// Based on Stanford's "Generative Agents" (Park et al., 2023).

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

	// Ledger KV persistence (optional, replaces file-based journal persistence)
	kvs *iledger.KVConfigStore
}

type LLMCallFunc func(ctx context.Context, system, user string) (string, error)

type ReverieConfig struct {
	Enabled         bool          `json:"enabled"`
	Interval        time.Duration `json:"interval"`         // how often to think (default: 30m)
	MaxJournal      int           `json:"max_journal"`      // max stored thoughts (default: 100)
	MinSignificance float64       `json:"min_significance"` // 0-1, threshold to deliver (default: 0.6)
	QuietStart      int           `json:"quiet_start"`      // quiet hours start (0-23, default: 22)
	QuietEnd        int           `json:"quiet_end"`        // quiet hours end (0-23, default: 7)
	SaveFile        string        `json:"save_file"`        // persist journal (default: data/reverie.json)
}

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

type ReverieAction struct {
	Type  string `json:"type"`  // "write_memory", "create_task", "update_profile"
	Key   string `json:"key"`   // context-dependent: fact text, task title, profile key
	Value string `json:"value"` // context-dependent: empty for memory, task description, profile value
}

type ActionRecord struct {
	ThoughtID string        `json:"thought_id"`
	Action    ReverieAction `json:"action"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	At        time.Time     `json:"at"`
}

func NewReverie(cfg ReverieConfig) *Reverie {
	r := &Reverie{
		cfg:       cfg,
		journal:   make([]Thought, 0),
		actionLog: make([]ActionRecord, 0),
	}
	r.loadJournal()
	return r
}

func (r *Reverie) SetLLMCall(fn LLMCallFunc)                 { r.llmCall = fn }
func (r *Reverie) SetRecall(fn func(query string) string)     { r.recall = fn }
func (r *Reverie) SetDeliver(fn func(thought Thought))        { r.deliver = fn }
func (r *Reverie) GetDeliver() func(Thought)                  { return r.deliver }
func (r *Reverie) SetOnThought(fn func(Thought))              { r.onThought = fn }
func (r *Reverie) SetEventBus(bus *ReverieEventBus)            { r.eventBus = bus }

// SetKVStore enables Ledger KV-backed persistence for the thought journal.
func (r *Reverie) SetKVStore(kvs *iledger.KVConfigStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kvs = kvs
	r.loadJournalFromKV()
}

func (r *Reverie) SetWriteMemory(fn func(ctx context.Context, fact string) error)        { r.writeMemory = fn }
func (r *Reverie) SetCreateTask(fn func(ctx context.Context, title, desc string) error)   { r.createTask = fn }
func (r *Reverie) SetUpdateProfile(fn func(ctx context.Context, key, value string) error) { r.updateProfile = fn }

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

func (r *Reverie) loadJournalFromKV() {
	if r.kvs == nil {
		return
	}
	var journal []Thought
	found, err := r.kvs.Get(context.Background(), "journal", &journal)
	if err != nil {
		slog.Warn("reverie: kv load failed", "err", err)
		return
	}
	if found && len(journal) > 0 {
		r.journal = journal
		slog.Info("reverie: loaded from Ledger KV", "thoughts", len(journal))
	}
}

func (r *Reverie) saveJournal() {
	r.mu.Lock()
	data, err := json.MarshalIndent(r.journal, "", "  ")
	kvs := r.kvs
	r.mu.Unlock()
	if err != nil {
		return
	}

	if kvs != nil {
		if err := kvs.Put(context.Background(), "journal", r.journal); err != nil {
			slog.Warn("reverie: kv save failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	if r.cfg.SaveFile == "" {
		return
	}
	if err := os.WriteFile(r.cfg.SaveFile, data, 0644); err != nil {
		slog.Warn("reverie: save journal failed", "err", err)
	}
}
