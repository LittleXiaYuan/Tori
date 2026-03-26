package channel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// State represents a channel's connection state.
type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateError        State = "error"
)

// ChannelStatus holds runtime status for a channel.
type ChannelStatus struct {
	Type       string    `json:"type"`
	State      State     `json:"state"`
	Error      string    `json:"error,omitempty"`
	LastActive time.Time `json:"last_active,omitempty"`
	MsgCount   int64     `json:"msg_count"`
}

// Observer is notified of channel lifecycle events.
type Observer interface {
	OnConnect(channelType string)
	OnDisconnect(channelType string, err error)
	OnMessage(channelType string, msg Message)
}

// LogObserver is a default observer that logs events.
type LogObserver struct{}

func (o *LogObserver) OnConnect(ct string)              { slog.Info("channel connected", "type", ct) }
func (o *LogObserver) OnDisconnect(ct string, err error) {
	if err != nil {
		slog.Warn("channel disconnected", "type", ct, "err", err)
	} else {
		slog.Info("channel disconnected", "type", ct)
	}
}
func (o *LogObserver) OnMessage(ct string, msg Message) {
	slog.Debug("channel message", "type", ct, "user", msg.UserName)
}

// Lifecycle manages channel connection states and auto-reconnection.
type Lifecycle struct {
	registry  *Registry
	observers []Observer
	statuses  map[string]*ChannelStatus
	mu        sync.RWMutex
	handler   func(Message) Reply
}

// NewLifecycle creates a lifecycle manager for the given registry.
func NewLifecycle(reg *Registry) *Lifecycle {
	return &Lifecycle{
		registry: reg,
		statuses: make(map[string]*ChannelStatus),
		observers: []Observer{&LogObserver{}},
	}
}

// AddObserver registers a lifecycle observer.
func (l *Lifecycle) AddObserver(obs Observer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.observers = append(l.observers, obs)
}

// StartAll launches all channels with lifecycle management.
// If the registry has a CommandInterceptor, slash commands are handled before reaching the planner.
func (l *Lifecycle) StartAll(ctx context.Context, handler func(Message) Reply) {
	wrapped := handler
	if l.registry.enricher != nil {
		wrapped = l.registry.enricher.Wrap(wrapped)
	}
	if l.registry.interceptor != nil {
		wrapped = l.registry.interceptor.Wrap(wrapped)
	}
	l.handler = wrapped
	for typ, ch := range l.registry.channels {
		l.mu.Lock()
		l.statuses[typ] = &ChannelStatus{Type: typ, State: StateConnecting}
		l.mu.Unlock()
		go l.runWithReconnect(ctx, typ, ch)
	}
}

// Status returns the current status of all channels.
func (l *Lifecycle) Status() []ChannelStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]ChannelStatus, 0, len(l.statuses))
	for _, s := range l.statuses {
		result = append(result, *s)
	}
	return result
}

// ChannelState returns the state of a specific channel.
func (l *Lifecycle) ChannelState(typ string) (ChannelStatus, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s, ok := l.statuses[typ]
	if !ok {
		return ChannelStatus{}, false
	}
	return *s, true
}

func (l *Lifecycle) runWithReconnect(ctx context.Context, typ string, ch Channel) {
	const maxBackoff = 60 * time.Second
	backoff := 2 * time.Second
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			l.updateState(typ, StateDisconnected, nil)
			return
		default:
		}

		l.updateState(typ, StateConnecting, nil)
		l.notifyConnect(typ)

		wrappedHandler := func(msg Message) Reply {
			l.recordMessage(typ, msg)
			l.notifyMessage(typ, msg)
			return l.handler(msg)
		}

		err := ch.Start(ctx, wrappedHandler)

		if ctx.Err() != nil {
			l.updateState(typ, StateDisconnected, nil)
			l.notifyDisconnect(typ, nil)
			return
		}

		attempt++
		l.updateState(typ, StateError, err)
		l.notifyDisconnect(typ, err)

		slog.Warn("channel disconnected, reconnecting",
			"type", typ,
			"attempt", attempt,
			"backoff", backoff,
			"err", err,
		)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (l *Lifecycle) updateState(typ string, state State, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	s, ok := l.statuses[typ]
	if !ok {
		s = &ChannelStatus{Type: typ}
		l.statuses[typ] = s
	}
	s.State = state
	if err != nil {
		s.Error = err.Error()
	} else {
		s.Error = ""
	}
	if state == StateConnected {
		s.LastActive = time.Now()
	}
}

func (l *Lifecycle) recordMessage(typ string, msg Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if s, ok := l.statuses[typ]; ok {
		s.MsgCount++
		s.LastActive = time.Now()
		s.State = StateConnected
	}
}

func (l *Lifecycle) notifyConnect(typ string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, obs := range l.observers {
		obs.OnConnect(typ)
	}
}

func (l *Lifecycle) notifyDisconnect(typ string, err error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, obs := range l.observers {
		obs.OnDisconnect(typ, err)
	}
}

func (l *Lifecycle) notifyMessage(typ string, msg Message) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, obs := range l.observers {
		obs.OnMessage(typ, msg)
	}
}

// NormalizeMessage cleans and standardizes an inbound message.
func NormalizeMessage(msg Message) Message {
	msg.Content = trimContent(msg.Content)
	if msg.ChannelType == "" {
		msg.ChannelType = "unknown"
	}
	if msg.Format == "" {
		msg.Format = "text"
	}
	return msg
}

func trimContent(s string) string {
	if len(s) > 32000 {
		s = s[:32000]
	}
	return s
}

// GenerateRouteKey builds a stable routing key for reply targeting.
func GenerateRouteKey(platform, channelID, userID string) string {
	return fmt.Sprintf("%s:%s:%s", platform, channelID, userID)
}
