package channel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ProgressTracker enhances IM UX by showing real-time execution progress.
// For channels that implement ProgressSender, it:
//  1. Sends a "thinking..." message immediately on receipt
//  2. Edits that message with accumulated step summaries
//  3. Sends the final reply as a new message
//
// For channels WITHOUT ProgressSender, it does nothing extra.
type ProgressTracker struct {
	Registry    *Registry
	MinInterval time.Duration // minimum interval between edits (default 800ms)
}

// progressSession tracks one in-flight request's progress state.
type progressSession struct {
	mu        sync.Mutex
	channel   ProgressSender
	target    string
	messageID string
	steps     []string
	lastEdit  time.Time
	interval  time.Duration
	done      bool
}

// AddStep records a step and edits the progress message if enough time has passed.
func (ps *progressSession) AddStep(ctx context.Context, icon, summary string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.done || ps.messageID == "" {
		return
	}

	line := fmt.Sprintf("%s %s", icon, summary)
	ps.steps = append(ps.steps, line)

	// Throttle edits to avoid rate limiting
	if time.Since(ps.lastEdit) < ps.interval {
		return
	}

	ps.editNow(ctx)
}

func (ps *progressSession) editNow(ctx context.Context) {
	text := ps.formatProgress()
	err := ps.channel.EditMessage(ctx, ps.target, ps.messageID, text)
	if err == nil {
		ps.lastEdit = time.Now()
	}
}

func (ps *progressSession) formatProgress() string {
	var b strings.Builder
	b.WriteString("⏳ Thinking...\n")
	for i, step := range ps.steps {
		if i == len(ps.steps)-1 {
			b.WriteString("└─ ")
		} else {
			b.WriteString("├─ ")
		}
		b.WriteString(step)
		b.WriteString("\n")
	}
	return b.String()
}

// Finalize edits the progress message one last time with all steps.
func (ps *progressSession) Finalize(ctx context.Context) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.done = true

	if ps.messageID == "" || len(ps.steps) == 0 {
		return
	}

	// Final edit with completed status
	var b strings.Builder
	b.WriteString("✅ Done\n")
	for i, step := range ps.steps {
		if i == len(ps.steps)-1 {
			b.WriteString("└─ ")
		} else {
			b.WriteString("├─ ")
		}
		b.WriteString(step)
		b.WriteString("\n")
	}
	_ = ps.channel.EditMessage(ctx, ps.target, ps.messageID, b.String())
}

// Wrap creates a handler wrapper that shows progress for ProgressSender channels.
// The returned handler also injects a ProgressSession into the message Extra map
// so that downstream handlers (gateway) can call AddStep during planner execution.
func (pt *ProgressTracker) Wrap(next func(Message) Reply) func(Message) Reply {
	if pt.MinInterval == 0 {
		pt.MinInterval = 800 * time.Millisecond
	}

	return func(msg Message) Reply {
		// Check if this channel supports progress sending
		ch, ok := pt.Registry.Get(msg.ChannelType)
		if !ok {
			return next(msg)
		}
		ps, ok := ch.(ProgressSender)
		if !ok {
			return next(msg)
		}

		ctx := context.Background()

		// Send initial "thinking..." message
		msgID, err := ps.SendAndGetID(ctx, msg.ChannelID, Reply{
			Content: "⏳ Thinking...",
			Format:  "text",
		})
		if err != nil || msgID == "" {
			// Fallback: no progress, just run normally
			return next(msg)
		}

		// Create session
		session := &progressSession{
			channel:   ps,
			target:    msg.ChannelID,
			messageID: msgID,
			interval:  pt.MinInterval,
			lastEdit:  time.Now(),
		}

		// Inject session ID into message Extra so gateway can use it
		if msg.Extra == nil {
			msg.Extra = make(map[string]string)
		}
		msg.Extra["_progress_msg_id"] = msgID

		// Store session in registry for access from gateway
		progressSessionStore.Store(progressKey(msg.ChannelType, msg.ChannelID, msgID), session)
		defer progressSessionStore.Delete(progressKey(msg.ChannelType, msg.ChannelID, msgID))

		// Run the actual handler
		reply := next(msg)

		// Finalize progress message
		session.Finalize(ctx)

		return reply
	}
}

// progressSessionStore holds active ProgressSessions keyed by channel:chat:msgID.
var progressSessionStore sync.Map

func progressKey(channelType, channelID, msgID string) string {
	return channelType + ":" + channelID + ":" + msgID
}

// GetProgressSession retrieves an active progress session for step updates.
// Call this from the gateway handler to add steps during planner execution.
func GetProgressSession(channelType, channelID, progressMsgID string) *progressSession {
	key := progressKey(channelType, channelID, progressMsgID)
	if v, ok := progressSessionStore.Load(key); ok {
		return v.(*progressSession)
	}
	return nil
}

// StepIcon maps event types to display icons.
func StepIcon(eventType string) string {
	switch eventType {
	case "thinking":
		return "💭"
	case "tool_start":
		return "🔧"
	case "tool_result":
		return "📄"
	case "reflect":
		return "🔍"
	case "plan":
		return "📋"
	default:
		return "▸"
	}
}
