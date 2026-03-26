package channel

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// GroupFilterConfig controls which group messages the bot responds to.
type GroupFilterConfig struct {
	AllowList []string `json:"allow_list,omitempty"` // only respond in these groups
	DenyList  []string `json:"deny_list,omitempty"`  // never respond in these groups
}

// LoadGroupFilterConfig loads group filter settings from env vars.
func LoadGroupFilterConfig() GroupFilterConfig {
	return GroupFilterConfig{} // default: no filtering
}

// SetGroupFilter attaches group filtering rules.
func (r *Registry) SetGroupFilter(cfg GroupFilterConfig) {
	r.groupFilter = &cfg
}

// SetEngagement attaches the engagement profile for group conversations.
func (r *Registry) SetEngagement(profile EngagementProfile) {
	r.engagement = &profile
}

// CurrentEngagement returns the current engagement profile.
func (r *Registry) CurrentEngagement() EngagementProfile {
	if r.engagement != nil {
		return *r.engagement
	}
	return EngagementProfile{Mode: "active"}
}

// Inbox returns the inbox channel for receiving group messages (may be nil).
func (r *Registry) Inbox() *InboxChannel {
	return r.inbox
}

// SetInbox sets the inbox channel.
func (r *Registry) SetInbox(inbox *InboxChannel) {
	r.inbox = inbox
}

// InboxChannel buffers recent group messages for heartbeat-style review.
type InboxChannel struct {
	messages []InboxMessage
	maxSize  int
}

// InboxMessage is a buffered group message.
type InboxMessage struct {
	ChannelType string
	ChannelID   string
	GroupID     string
	UserName    string
	Content     string
	Time        time.Time
}

// NewInbox creates a new inbox buffer.
func NewInbox(maxSize int) *InboxChannel {
	return &InboxChannel{maxSize: maxSize}
}

// Add adds a message to the inbox buffer.
func (ib *InboxChannel) Add(msg InboxMessage) {
	ib.messages = append(ib.messages, msg)
	if len(ib.messages) > ib.maxSize {
		ib.messages = ib.messages[len(ib.messages)-ib.maxSize:]
	}
}

// Flush returns and clears buffered messages.
func (ib *InboxChannel) Flush() []InboxMessage {
	msgs := ib.messages
	ib.messages = nil
	return msgs
}

// Peek returns buffered messages without clearing.
func (ib *InboxChannel) Peek() []InboxMessage {
	return ib.messages
}

// Drain returns and clears buffered messages (alias for Flush).
func (ib *InboxChannel) Drain() []InboxMessage {
	return ib.Flush()
}

// FormatForContext formats buffered messages as a context string for the planner.
func (ib *InboxChannel) FormatForContext() string {
	if len(ib.messages) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, m := range ib.messages {
		sb.WriteString(m.UserName + ": " + m.Content + "\n")
	}
	return sb.String()
}

// GroupHeartbeat manages periodic group engagement checks.
type GroupHeartbeat struct {
	inbox       *InboxChannel
	profile     *EngagementProfile
	registry    *Registry
	callback    func(ctx context.Context, inboxContext string) (string, string, string)
	interval    time.Duration
	cancel      context.CancelFunc
}

// NewGroupHeartbeat creates a heartbeat that checks group inboxes periodically.
func NewGroupHeartbeat(
	inbox *InboxChannel,
	profile *EngagementProfile,
	registry *Registry,
	callback func(ctx context.Context, inboxContext string) (string, string, string),
) *GroupHeartbeat {
	interval := 30 * time.Minute
	if profile != nil && profile.HeartbeatInterval > 0 {
		interval = profile.HeartbeatInterval
	}
	// Also check env var
	if v := os.Getenv("GROUP_HEARTBEAT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = time.Duration(n) * time.Minute
		}
	}
	return &GroupHeartbeat{
		inbox:    inbox,
		profile:  profile,
		registry: registry,
		callback: callback,
		interval: interval,
	}
}

// Start begins the heartbeat loop. Runs until ctx is cancelled.
func (hb *GroupHeartbeat) Start(ctx context.Context) {
	ctx, hb.cancel = context.WithCancel(ctx)
	ticker := time.NewTicker(hb.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hb.tick(ctx)
		}
	}
}

func (hb *GroupHeartbeat) tick(ctx context.Context) {
	if hb.inbox == nil {
		return
	}
	msgs := hb.inbox.Flush()
	if len(msgs) == 0 {
		return
	}
	// Build context string from buffered messages
	var context string
	for _, m := range msgs {
		context += m.UserName + ": " + m.Content + "\n"
	}
	reply, target, channelType := hb.callback(ctx, context)
	if reply == "" || target == "" || channelType == "" {
		return
	}
	ch, ok := hb.registry.Get(channelType)
	if !ok {
		return
	}
	if err := ch.Send(ctx, target, Reply{Content: reply, Format: "text"}); err != nil {
		slog.Warn("group heartbeat send failed", "err", err, "channel", channelType)
	}
}
