package browser

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Notifier — multi-channel OPP & browser event notifications
//
// Dispatches browser events (screenshots, OPP problems, action logs)
// to configured channels: WebUI (SSE), IM (Feishu), etc.
// ──────────────────────────────────────────────

// EventType for browser notifications.
type EventType string

const (
	EventScreenshot EventType = "browser.screenshot" // screenshot frame (base64)
	EventAction     EventType = "browser.action"     // agent action (click, type, etc.)
	EventProblem    EventType = "browser.problem"     // OPP PROBLEM (needs human)
	EventDecided    EventType = "browser.decided"     // user responded to PROBLEM
	EventResult     EventType = "browser.result"      // task completed
)

// BrowserEvent is the payload sent to notification channels.
type BrowserEvent struct {
	Type      EventType `json:"type"`
	Timestamp int64     `json:"timestamp"`
	TaskID    string    `json:"task_id,omitempty"`
	Data      any       `json:"data"`
}

// ProblemData describes an OPP PROBLEM requiring human intervention.
type ProblemData struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Screenshot  string `json:"screenshot,omitempty"` // base64
	URL         string `json:"url,omitempty"`
	Options     []ProblemOption `json:"options,omitempty"`
}

// ProblemOption is a selectable response for an OPP PROBLEM.
type ProblemOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// DecideRequest is the user's response to an OPP PROBLEM.
type DecideRequest struct {
	ProblemID string `json:"problem_id"`
	Decision  string `json:"decision"` // option key or free text
}

// NotifyChannel sends events to a specific destination.
type NotifyChannel interface {
	Name() string
	Send(ctx context.Context, event BrowserEvent) error
}

// Notifier dispatches browser events to multiple channels.
type Notifier struct {
	mu       sync.RWMutex
	channels []NotifyChannel
	pending  map[string]*ProblemData // problem ID → problem
	pendMu   sync.RWMutex
	decideCh map[string]chan string  // problem ID → decision channel
}

// NewNotifier creates a notifier with the given channels.
func NewNotifier(channels ...NotifyChannel) *Notifier {
	return &Notifier{
		channels: channels,
		pending:  make(map[string]*ProblemData),
		decideCh: make(map[string]chan string),
	}
}

// AddChannel adds a notification channel at runtime.
func (n *Notifier) AddChannel(ch NotifyChannel) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.channels = append(n.channels, ch)
}

// Broadcast sends an event to all channels.
func (n *Notifier) Broadcast(ctx context.Context, event BrowserEvent) {
	event.Timestamp = time.Now().UnixMilli()
	n.mu.RLock()
	chs := make([]NotifyChannel, len(n.channels))
	copy(chs, n.channels)
	n.mu.RUnlock()

	for _, ch := range chs {
		go func(c NotifyChannel) {
			if err := c.Send(ctx, event); err != nil {
				slog.Warn("browser/notifier: send failed", "channel", c.Name(), "err", err)
			}
		}(ch)
	}
}

// BroadcastScreenshot sends a screenshot event.
func (n *Notifier) BroadcastScreenshot(ctx context.Context, b64 string) {
	n.Broadcast(ctx, BrowserEvent{
		Type: EventScreenshot,
		Data: map[string]string{"image": b64},
	})
}

// BroadcastAction sends an action event (e.g., "clicking button #submit").
func (n *Notifier) BroadcastAction(ctx context.Context, action, detail string) {
	n.Broadcast(ctx, BrowserEvent{
		Type: EventAction,
		Data: map[string]string{"action": action, "detail": detail},
	})
}

// RaiseProblem broadcasts a PROBLEM and waits for human decision.
// Returns the decision string or error on context cancel.
func (n *Notifier) RaiseProblem(ctx context.Context, problem ProblemData) (string, error) {
	// Register pending problem
	ch := make(chan string, 1)
	n.pendMu.Lock()
	n.pending[problem.ID] = &problem
	n.decideCh[problem.ID] = ch
	n.pendMu.Unlock()

	defer func() {
		n.pendMu.Lock()
		delete(n.pending, problem.ID)
		delete(n.decideCh, problem.ID)
		n.pendMu.Unlock()
	}()

	// Broadcast to all channels
	n.Broadcast(ctx, BrowserEvent{
		Type: EventProblem,
		Data: problem,
	})

	// Wait for decision or cancel
	select {
	case decision := <-ch:
		n.Broadcast(ctx, BrowserEvent{
			Type: EventDecided,
			Data: map[string]string{"problem_id": problem.ID, "decision": decision},
		})
		return decision, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// ResolveProblem submits a human decision for a pending problem.
func (n *Notifier) ResolveProblem(problemID, decision string) bool {
	n.pendMu.RLock()
	ch, ok := n.decideCh[problemID]
	n.pendMu.RUnlock()
	if !ok {
		return false
	}
	select {
	case ch <- decision:
		return true
	default:
		return false
	}
}

// PendingProblems returns all unresolved problems.
func (n *Notifier) PendingProblems() []ProblemData {
	n.pendMu.RLock()
	defer n.pendMu.RUnlock()
	out := make([]ProblemData, 0, len(n.pending))
	for _, p := range n.pending {
		out = append(out, *p)
	}
	return out
}

// ──────────────────────────────────────────────
// SSE Channel — broadcasts events via Gateway SSEBroker
// ──────────────────────────────────────────────

// SSEChannel sends browser events to the SSE broker.
type SSEChannel struct {
	broadcast func(eventType string, data any) // gateway.SSEBroker.Broadcast wrapper
}

// NewSSEChannel creates a channel that pushes to the SSE broker.
// broadcastFn should call sseBroker.Broadcast(SSEEvent{Type: t, Data: d}).
func NewSSEChannel(broadcastFn func(string, any)) *SSEChannel {
	return &SSEChannel{broadcast: broadcastFn}
}

func (c *SSEChannel) Name() string { return "webui_sse" }

func (c *SSEChannel) Send(_ context.Context, event BrowserEvent) error {
	if c.broadcast == nil {
		return nil
	}
	c.broadcast(string(event.Type), event)
	return nil
}

// ──────────────────────────────────────────────
// IM Channel — sends OPP problems to IM (Feishu, etc.)
// ──────────────────────────────────────────────

// IMChannel sends critical events (problems, results) to IM.
type IMChannel struct {
	sendFn func(ctx context.Context, text string) error
}

// NewIMChannel creates an IM notification channel.
// sendFn should send a text message to the configured IM (Feishu, Slack, etc.).
func NewIMChannel(sendFn func(context.Context, string) error) *IMChannel {
	return &IMChannel{sendFn: sendFn}
}

func (c *IMChannel) Name() string { return "im" }

func (c *IMChannel) Send(ctx context.Context, event BrowserEvent) error {
	// Only send problems and results to IM (not screenshots)
	switch event.Type {
	case EventProblem:
		data, _ := json.Marshal(event.Data)
		return c.sendFn(ctx, "🚨 浏览器需要你的帮助:\n"+string(data))
	case EventResult:
		data, _ := json.Marshal(event.Data)
		return c.sendFn(ctx, "✅ 浏览器任务完成:\n"+string(data))
	default:
		return nil // skip screenshots/actions for IM
	}
}
