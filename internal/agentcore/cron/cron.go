package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Schedule types
// ──────────────────────────────────────────────

// ScheduleType determines how a job is triggered.
type ScheduleType string

const (
	ScheduleAt    ScheduleType = "at"    // one-shot ISO 8601 timestamp
	ScheduleEvery ScheduleType = "every" // fixed interval in milliseconds
	ScheduleCron  ScheduleType = "cron"  // 5-field cron expression
)

// Schedule defines when a job fires.
type Schedule struct {
	Type     ScheduleType `json:"type"`
	At       *time.Time   `json:"at,omitempty"`        // for "at"
	EveryMs  int64        `json:"every_ms,omitempty"`  // for "every"
	CronExpr string       `json:"cron_expr,omitempty"` // for "cron"
	Timezone string       `json:"timezone,omitempty"`  // IANA timezone for cron
}

// ──────────────────────────────────────────────
// Payload & delivery
// ──────────────────────────────────────────────

// PayloadKind determines what the job does when it fires.
type PayloadKind string

const (
	PayloadSystemEvent PayloadKind = "systemEvent" // enqueue system event
	PayloadAgentTurn   PayloadKind = "agentTurn"   // run a dedicated agent turn
)

// Payload defines the action taken when a job fires.
type Payload struct {
	Kind    PayloadKind    `json:"kind"`
	Message string         `json:"message,omitempty"` // prompt or event text
	Data    map[string]any `json:"data,omitempty"`    // arbitrary key-value
}

// DeliveryMode controls how output is delivered.
type DeliveryMode string

const (
	DeliveryAnnounce DeliveryMode = "announce" // broadcast to session
	DeliveryWebhook  DeliveryMode = "webhook"  // POST to webhook URL
	DeliveryNone     DeliveryMode = "none"     // discard
)

// ──────────────────────────────────────────────
// Job
// ──────────────────────────────────────────────

// Job represents a scheduled task.
type Job struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Schedule      Schedule     `json:"schedule"`
	Payload       Payload      `json:"payload"`
	AgentID       string       `json:"agent_id,omitempty"`       // target agent
	SessionTarget string       `json:"session_target,omitempty"` // target session
	Delivery      DeliveryMode `json:"delivery,omitempty"`
	Enabled       bool         `json:"enabled"`
	CreatedAt     time.Time    `json:"created_at"`
	LastRunAt     *time.Time   `json:"last_run_at,omitempty"`
	NextRunAt     *time.Time   `json:"next_run_at,omitempty"`
	RunCount      int          `json:"run_count"`
}

// ──────────────────────────────────────────────
// Run record
// ──────────────────────────────────────────────

// RunStatus is the result of a single job execution.
type RunStatus string

const (
	RunSuccess RunStatus = "success"
	RunFailed  RunStatus = "failed"
	RunSkipped RunStatus = "skipped"
)

// RunRecord captures one execution of a job.
type RunRecord struct {
	JobID     string    `json:"job_id"`
	RunID     string    `json:"run_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Status    RunStatus `json:"status"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// ──────────────────────────────────────────────
// JobHandler — callback invoked when a job fires
// ──────────────────────────────────────────────

// JobHandler is called by the manager when a job fires.
// Returning an error marks the run as failed.
type JobHandler func(ctx context.Context, job *Job) (output string, err error)

// SessionFactory creates an isolated session ID for an agentTurn job execution.
// If nil, the job's SessionTarget is used directly.
type SessionFactory func(job *Job, runID string) string

// ──────────────────────────────────────────────
// Manager
// ──────────────────────────────────────────────

// Manager schedules and persists cron jobs.
type Manager struct {
	mu             sync.RWMutex
	jobs           map[string]*Job
	timers         map[string]*time.Timer
	handler        JobHandler
	sessionFactory SessionFactory
	dataDir        string
	ctx            context.Context
	cancel         context.CancelFunc
	stopped        bool
}

// NewManager creates a cron manager that persists jobs to dataDir.
func NewManager(dataDir string, handler JobHandler) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		jobs:    make(map[string]*Job),
		timers:  make(map[string]*time.Timer),
		handler: handler,
		dataDir: dataDir,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start loads persisted jobs and arms their timers.
func (m *Manager) Start() error {
	if err := m.load(); err != nil {
		slog.Warn("cron: load failed, starting fresh", "err", err)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, j := range m.jobs {
		if j.Enabled {
			if _, armed := m.timers[j.ID]; !armed {
				m.arm(j)
			}
		}
	}
	slog.Info("cron: started", "jobs", len(m.jobs))
	return nil
}

// Stop cancels all timers and persists state.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	m.cancel()
	for id, t := range m.timers {
		t.Stop()
		delete(m.timers, id)
	}
	if err := m.persist(); err != nil {
		slog.Error("cron: persist on stop", "err", err)
	}
	slog.Info("cron: stopped")
}

// ──────────────────────────────────────────────
// CRUD
// ──────────────────────────────────────────────

// Add creates and arms a new job. Returns the job ID.
func (m *Manager) Add(name string, sched Schedule, payload Payload, opts ...JobOption) (string, error) {
	j := &Job{
		ID:        uuid.New().String(),
		Name:      name,
		Schedule:  sched,
		Payload:   payload,
		Enabled:   true,
		CreatedAt: time.Now(),
		Delivery:  DeliveryNone,
	}
	for _, o := range opts {
		o(j)
	}
	// Compute next run
	next := m.nextRun(j)
	j.NextRunAt = next

	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[j.ID] = j
	if j.Enabled {
		m.arm(j)
	}
	if err := m.persist(); err != nil {
		slog.Warn("cron: persist after add", "err", err)
	}
	slog.Info("cron: added job", "id", j.ID, "name", j.Name, "schedule", j.Schedule.Type)
	return j.ID, nil
}

// JobOption configures optional job fields.
type JobOption func(*Job)

func WithAgent(agentID string) JobOption    { return func(j *Job) { j.AgentID = agentID } }
func WithSession(target string) JobOption   { return func(j *Job) { j.SessionTarget = target } }
func WithDelivery(d DeliveryMode) JobOption { return func(j *Job) { j.Delivery = d } }

// SetSessionFactory sets a callback used to create isolated session IDs for
// agentTurn jobs. Each execution gets its own session to avoid polluting
// user conversation history.
func (m *Manager) SetSessionFactory(sf SessionFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionFactory = sf
}

// Update modifies a job. Only non-zero fields in the update are applied.
func (m *Manager) Update(id string, name *string, sched *Schedule, payload *Payload, enabled *bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("cron: job %q not found", id)
	}
	if name != nil {
		j.Name = *name
	}
	if sched != nil {
		j.Schedule = *sched
	}
	if payload != nil {
		j.Payload = *payload
	}
	if enabled != nil {
		j.Enabled = *enabled
	}
	// Re-arm
	if t, ok := m.timers[id]; ok {
		t.Stop()
		delete(m.timers, id)
	}
	j.NextRunAt = m.nextRun(j)
	if j.Enabled {
		m.arm(j)
	}
	return m.persist()
}

// Remove deletes a job.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.jobs[id]; !ok {
		return fmt.Errorf("cron: job %q not found", id)
	}
	if t, ok := m.timers[id]; ok {
		t.Stop()
		delete(m.timers, id)
	}
	delete(m.jobs, id)
	return m.persist()
}

// Get returns a copy of a job.
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, false
	}
	cp := *j
	return &cp, true
}

// List returns all jobs.
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		cp := *j
		out = append(out, &cp)
	}
	return out
}

// RunNow forces immediate execution of a job.
func (m *Manager) RunNow(id string) (*RunRecord, error) {
	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("cron: job %q not found", id)
	}
	return m.execute(j), nil
}

// ──────────────────────────────────────────────
// Scheduling internals
// ──────────────────────────────────────────────

// arm sets a timer for the next firing of the job. Must be called with mu held or inside a lock.
func (m *Manager) arm(j *Job) {
	dur := m.durationUntilNext(j)
	if dur < 0 {
		return // one-shot in the past
	}
	t := time.AfterFunc(dur, func() {
		if m.stopped {
			return
		}
		m.execute(j)
		// Re-arm for recurring schedules
		if j.Schedule.Type != ScheduleAt {
			m.mu.Lock()
			j.NextRunAt = m.nextRun(j)
			if j.Enabled {
				m.arm(j)
			}
			m.mu.Unlock()
		} else {
			// One-shot: disable after firing
			m.mu.Lock()
			j.Enabled = false
			j.NextRunAt = nil
			m.persist()
			m.mu.Unlock()
		}
	})
	m.timers[j.ID] = t
}

func (m *Manager) durationUntilNext(j *Job) time.Duration {
	n := m.nextRun(j)
	if n == nil {
		return -1
	}
	d := time.Until(*n)
	if d < 0 {
		d = 0
	}
	return d
}

func (m *Manager) nextRun(j *Job) *time.Time {
	now := time.Now()
	switch j.Schedule.Type {
	case ScheduleAt:
		if j.Schedule.At != nil {
			t := *j.Schedule.At
			return &t
		}
	case ScheduleEvery:
		if j.Schedule.EveryMs <= 0 {
			return nil
		}
		d := time.Duration(j.Schedule.EveryMs) * time.Millisecond
		var base time.Time
		if j.LastRunAt != nil {
			base = *j.LastRunAt
		} else {
			base = j.CreatedAt
		}
		next := base.Add(d)
		if next.Before(now) {
			next = now.Add(d)
		}
		return &next
	case ScheduleCron:
		next := nextCronTime(j.Schedule.CronExpr, j.Schedule.Timezone, now)
		return next
	}
	return nil
}

func (m *Manager) execute(j *Job) *RunRecord {
	rec := &RunRecord{
		JobID:     j.ID,
		RunID:     uuid.New().String(),
		StartedAt: time.Now(),
	}

	if m.handler == nil {
		rec.Status = RunSkipped
		rec.EndedAt = time.Now()
		return rec
	}

	// For agentTurn jobs: create an isolated session so cron output
	// doesn't pollute real user conversations.
	m.mu.RLock()
	sf := m.sessionFactory
	m.mu.RUnlock()
	if j.Payload.Kind == PayloadAgentTurn && sf != nil {
		j.SessionTarget = sf(j, rec.RunID)
	}

	output, err := m.handler(m.ctx, j)
	rec.EndedAt = time.Now()
	rec.Output = output
	if err != nil {
		rec.Status = RunFailed
		rec.Error = err.Error()
		slog.Warn("cron: job failed", "id", j.ID, "name", j.Name, "err", err)
	} else {
		rec.Status = RunSuccess
		slog.Info("cron: job executed", "id", j.ID, "name", j.Name)
	}

	// Update job state
	m.mu.Lock()
	now := time.Now()
	j.LastRunAt = &now
	j.RunCount++
	m.mu.Unlock()

	// Append run history
	m.appendRunRecord(rec)
	return rec
}

// ──────────────────────────────────────────────
// Persistence
// ──────────────────────────────────────────────

func (m *Manager) jobsFile() string {
	return filepath.Join(m.dataDir, "cron", "jobs.json")
}

func (m *Manager) runsDir() string {
	return filepath.Join(m.dataDir, "cron", "runs")
}

func (m *Manager) persist() error {
	dir := filepath.Dir(m.jobsFile())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	jobs := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.jobsFile(), data, 0o644)
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.jobsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var jobs []*Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return err
	}
	for _, j := range jobs {
		m.jobs[j.ID] = j
	}
	return nil
}

func (m *Manager) appendRunRecord(rec *RunRecord) {
	dir := m.runsDir()
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, rec.JobID+".jsonl")
	line, _ := json.Marshal(rec)
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("cron: write run record", "err", err)
		return
	}
	defer f.Close()
	f.Write(line)
}

// RunHistory returns the last N run records for a job.
func (m *Manager) RunHistory(jobID string, limit int) ([]RunRecord, error) {
	path := filepath.Join(m.runsDir(), jobID+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []RunRecord
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var r RunRecord
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		records = append(records, r)
	}
	// Return last N
	if limit > 0 && len(records) > limit {
		records = records[len(records)-limit:]
	}
	return records, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// ──────────────────────────────────────────────
// Minimal cron expression parser (5-field)
// Supports: minute hour day-of-month month day-of-week
// Fields: * or number or */step
// ──────────────────────────────────────────────

func nextCronTime(expr, tz string, after time.Time) *time.Time {
	loc := time.Local
	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	after = after.In(loc)

	fields := splitFields(expr)
	if len(fields) < 5 {
		return nil
	}

	// Brute-force: check each minute in the next 366 days
	t := after.Truncate(time.Minute).Add(time.Minute)
	end := after.Add(366 * 24 * time.Hour)
	for t.Before(end) {
		if matchField(fields[0], t.Minute()) &&
			matchField(fields[1], t.Hour()) &&
			matchField(fields[2], t.Day()) &&
			matchField(fields[3], int(t.Month())) &&
			matchField(fields[4], int(t.Weekday())) {
			result := t.In(time.Local)
			return &result
		}
		t = t.Add(time.Minute)
	}
	return nil
}

func splitFields(s string) []string {
	var fields []string
	field := ""
	for _, c := range s {
		if c == ' ' || c == '\t' {
			if field != "" {
				fields = append(fields, field)
				field = ""
			}
		} else {
			field += string(c)
		}
	}
	if field != "" {
		fields = append(fields, field)
	}
	return fields
}

func matchField(field string, value int) bool {
	if field == "*" {
		return true
	}
	// */step
	if len(field) > 2 && field[:2] == "*/" {
		step := atoi(field[2:])
		if step <= 0 {
			return false
		}
		return value%step == 0
	}
	// exact number
	return atoi(field) == value
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return int(math.MinInt32)
		}
		n = n*10 + int(c-'0')
	}
	return n
}
