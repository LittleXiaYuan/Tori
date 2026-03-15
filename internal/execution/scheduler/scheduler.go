package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is a scheduled task.
type Job struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	TenantID string        `json:"tenant_id"`
	Interval time.Duration `json:"interval"`
	Prompt   string        `json:"prompt"` // message to send to planner
	Enabled  bool          `json:"enabled"`
	LastRun  time.Time     `json:"last_run"`
	NextRun  time.Time     `json:"next_run"`
}

// Handler is called when a job triggers.
type Handler func(ctx context.Context, job Job)

// Scheduler manages periodic tasks.
type Scheduler struct {
	mu      sync.RWMutex
	jobs    map[string]*Job
	handler Handler
	stop    chan struct{}
}

// New creates a scheduler.
func New(handler Handler) *Scheduler {
	return &Scheduler{
		jobs:    make(map[string]*Job),
		handler: handler,
		stop:    make(chan struct{}),
	}
}

// Add registers a new job.
func (s *Scheduler) Add(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job.NextRun = time.Now().Add(job.Interval)
	job.Enabled = true
	s.jobs[job.ID] = &job
	slog.Info("scheduler: job added", "id", job.ID, "name", job.Name, "interval", job.Interval)
}

// Remove deletes a job.
func (s *Scheduler) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
}

// List returns all jobs.
func (s *Scheduler) List() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, *j)
	}
	return out
}

// Enable or disable a job.
func (s *Scheduler) SetEnabled(id string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Enabled = enabled
	}
}

// Start begins the scheduler loop.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case now := <-ticker.C:
			s.tick(ctx, now)
		}
	}
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	s.mu.Lock()
	var due []*Job
	for _, j := range s.jobs {
		if j.Enabled && now.After(j.NextRun) {
			due = append(due, j)
		}
	}
	for _, j := range due {
		j.LastRun = now
		j.NextRun = now.Add(j.Interval)
	}
	s.mu.Unlock()

	for _, j := range due {
		slog.Info("scheduler: triggering", "job", j.Name)
		go s.handler(ctx, *j)
	}
}
