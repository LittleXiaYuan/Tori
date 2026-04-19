package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type LaunchTask struct {
	TaskID      string
	ProjectID   string
	Description string
	WorkDir     string
	Steps       []string
	MCPEndpoint string
	Rules       string
}

type LaunchResult struct {
	SessionID string
	WorkerID  string
	PID       int
	StartedAt time.Time
}

type ProgressEvent struct {
	SessionID string
	Type      string // "output" | "error" | "done"
	Data      string
	Timestamp time.Time
}

type WorkerAdapter interface {
	Name() string
	Available() bool
	Launch(ctx context.Context, task LaunchTask) (*LaunchResult, error)
	Monitor(ctx context.Context, sessionID string) (<-chan ProgressEvent, error)
	Stop(sessionID string) error
}

type Launcher struct {
	mu       sync.RWMutex
	adapters map[string]WorkerAdapter
	sessions map[string]*activeSession
}

type activeSession struct {
	SessionID   string
	AdapterName string
	TaskID      string
	StartedAt   time.Time
	cancel      context.CancelFunc
}

func NewLauncher() *Launcher {
	return &Launcher{
		adapters: make(map[string]WorkerAdapter),
		sessions: make(map[string]*activeSession),
	}
}

func (l *Launcher) RegisterAdapter(a WorkerAdapter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.adapters[a.Name()] = a
}

func (l *Launcher) AvailableAdapters() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []string
	for name, a := range l.adapters {
		if a.Available() {
			out = append(out, name)
		}
	}
	return out
}

func (l *Launcher) Launch(ctx context.Context, adapterName string, task LaunchTask) (*LaunchResult, error) {
	l.mu.RLock()
	a, ok := l.adapters[adapterName]
	l.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("adapter %q not registered", adapterName)
	}
	if !a.Available() {
		return nil, fmt.Errorf("adapter %q not available (tool not installed?)", adapterName)
	}

	childCtx, cancel := context.WithCancel(ctx)
	result, err := a.Launch(childCtx, task)
	if err != nil {
		cancel()
		return nil, err
	}

	l.mu.Lock()
	l.sessions[result.SessionID] = &activeSession{
		SessionID:   result.SessionID,
		AdapterName: adapterName,
		TaskID:      task.TaskID,
		StartedAt:   result.StartedAt,
		cancel:      cancel,
	}
	l.mu.Unlock()

	return result, nil
}

func (l *Launcher) StopSession(sessionID string) error {
	l.mu.Lock()
	s, ok := l.sessions[sessionID]
	if ok {
		s.cancel()
		delete(l.sessions, sessionID)
	}
	l.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}

	l.mu.RLock()
	a, aOk := l.adapters[s.AdapterName]
	l.mu.RUnlock()
	if aOk {
		return a.Stop(sessionID)
	}
	return nil
}

func (l *Launcher) ActiveSessions() []activeSession {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]activeSession, 0, len(l.sessions))
	for _, s := range l.sessions {
		out = append(out, *s)
	}
	return out
}
