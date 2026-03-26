package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type InterruptKind string

const (
	InterruptSupplement InterruptKind = "supplement"
	InterruptCorrection InterruptKind = "correction"
	InterruptNewTask    InterruptKind = "new_task"
	InterruptNone       InterruptKind = "none"
)

type RunState struct {
	SessionID string
	StartedAt time.Time
	Cancel    context.CancelFunc
	mu          sync.Mutex
	interrupted bool
	interruptKind InterruptKind
	interruptMsg  string
	supplements   []string
}

func (rs *RunState) Interrupt(kind InterruptKind, msg string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.interrupted = true
	rs.interruptKind = kind
	rs.interruptMsg = msg
	if kind == InterruptSupplement {
		rs.supplements = append(rs.supplements, msg)
	}
	slog.Info("session: interrupt set", "session", rs.SessionID, "kind", kind)
}

func (rs *RunState) CheckInterrupt() (bool, InterruptKind, string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if !rs.interrupted {
		return false, InterruptNone, ""
	}
	kind := rs.interruptKind
	msg := rs.interruptMsg
	rs.interrupted = false
	rs.interruptKind = InterruptNone
	rs.interruptMsg = ""
	return true, kind, msg
}

func (rs *RunState) DrainSupplements() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	s := rs.supplements
	rs.supplements = nil
	return s
}

type Runner struct {
	mu   sync.RWMutex
	runs map[string]*RunState
}

func NewRunner() *Runner {
	return &Runner{runs: make(map[string]*RunState)}
}

func (r *Runner) StartRun(ctx context.Context, sessionID string) (*RunState, context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.runs[sessionID]; ok && existing.Cancel != nil {
		return existing, ctx, fmt.Errorf("session %s already running", sessionID)
	}
	runCtx, cancel := context.WithCancel(ctx)
	rs := &RunState{SessionID: sessionID, StartedAt: time.Now(), Cancel: cancel}
	r.runs[sessionID] = rs
	return rs, runCtx, nil
}

func (r *Runner) EndRun(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.runs, sessionID)
}

func (r *Runner) IsRunning(sessionID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.runs[sessionID]
	return ok
}

func (r *Runner) GetRun(sessionID string) *RunState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runs[sessionID]
}

func (r *Runner) HandleMidRunMessage(sessionID, message string) InterruptKind {
	rs := r.GetRun(sessionID)
	if rs == nil {
		return InterruptNone
	}
	kind := ClassifyInterrupt(message)
	switch kind {
	case InterruptSupplement:
		rs.Interrupt(InterruptSupplement, message)
	case InterruptCorrection:
		rs.Interrupt(InterruptCorrection, message)
	case InterruptNewTask:
		return InterruptNewTask
	}
	return kind
}

func ClassifyInterrupt(msg string) InterruptKind {
	lower := strings.ToLower(msg)
	trimmed := strings.TrimSpace(msg)
	for _, s := range []string{"算了", "不要了", "换一个", "停", "取消", "重新", "cancel", "stop", "abort"} {
		if strings.Contains(lower, s) {
			return InterruptCorrection
		}
	}
	for _, s := range []string{"对了", "补充", "顺便", "还有", "密码是", "btw", "also", "by the way"} {
		if strings.Contains(lower, s) {
			return InterruptSupplement
		}
	}
	if len([]rune(trimmed)) < 30 {
		return InterruptSupplement
	}
	return InterruptNewTask
}
