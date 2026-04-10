package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	rdebug "runtime/debug"
	"sync"
	"time"

	iledger "yunque-agent/internal/ledger"
)

// GroupInfo describes a group/guild/room the bot is a member of.
type GroupInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	ChannelType string `json:"channel_type"` // "telegram", "discord", etc.
	ChatType    string `json:"chat_type"`    // "group", "supergroup", "guild", "room"
	MemberCount int    `json:"member_count,omitempty"`
	LastActive  string `json:"last_active,omitempty"` // RFC3339
}

// GroupLister is an optional interface for channels that can list the groups/guilds the bot is in.
type GroupLister interface {
	// ListGroups returns all groups/guilds the bot is currently a member of.
	ListGroups(ctx context.Context) ([]GroupInfo, error)
}

// Message represents an incoming message from any channel.
type Message struct {
	ChannelType string            `json:"channel_type"` // "feishu", "telegram", "qq", "wechat", "slack", "http"
	ChannelID   string            `json:"channel_id"`   // group/chat ID
	UserID      string            `json:"user_id"`
	UserName    string            `json:"user_name"`
	Content     string            `json:"content"`
	Format      string            `json:"format,omitempty"`   // "text", "markdown", "html"
	ReplyTo     string            `json:"reply_to,omitempty"` // message ID being replied to
	Extra       map[string]string `json:"extra,omitempty"`    // channel-specific metadata
	Rich        *RichMessage      `json:"-"`                  // structured rich message (optional)
}

// Reply is the agent's response to send back.
type Reply struct {
	Content string       `json:"content"`
	Format  string       `json:"format"` // "text", "markdown", "html"
	ReplyTo string       `json:"reply_to,omitempty"`
	Rich    *RichMessage `json:"-"` // structured rich reply (optional)
}

// Channel is the interface for messaging platforms.
type Channel interface {
	// Type returns the channel type identifier.
	Type() string
	// Start begins listening for messages (blocking).
	Start(ctx context.Context, handler func(Message) Reply) error
	// Send pushes a proactive message to a target.
	Send(ctx context.Context, target string, reply Reply) error
}

// ProgressSender is an optional interface for channels that support
// multi-step output via message editing (e.g., Telegram editMessageText).
// This enables the "Agent thinking process" UX in IM channels:
//   1. Send initial "thinking..." message → get messageID
//   2. Edit message with progress updates
//   3. Send final reply as a new message
type ProgressSender interface {
	// SendAndGetID sends a message and returns its platform message ID.
	SendAndGetID(ctx context.Context, target string, reply Reply) (messageID string, err error)
	// EditMessage updates an existing message by ID.
	EditMessage(ctx context.Context, target string, messageID string, content string) error
}

// Reactor is an optional interface for channels that support emoji reactions on messages.
// Channels that implement this can add emoji/sticker reactions to incoming messages,
// similar to Telegram's setMessageReaction or Discord's addReaction.
type Reactor interface {
	// React adds an emoji reaction to a specific message.
	// messageID is the platform message ID, emoji is a unicode emoji or platform-specific emoji identifier.
	// Pass empty emoji to remove the bot's reaction.
	React(ctx context.Context, target string, messageID string, emoji string) error
}

// StickerSender is an optional interface for channels that can send sticker messages natively.
// Channels that implement this support sending stickers directly (Telegram sendSticker, LINE sticker message, etc.)
// instead of falling back to image or text.
type StickerSender interface {
	// SendSticker sends a sticker to the target chat.
	SendSticker(ctx context.Context, target string, sticker *StickerComponent) error
}

// Registry manages multiple channels.
type Registry struct {
	channels        map[string]Channel
	tracker         *GroupTracker
	groupFilter     *GroupFilterConfig
	engagement      *EngagementProfile
	inbox           *InboxChannel
	interceptor     *CommandInterceptor                  // universal command handler
	enricher        *StickerEnricher                     // auto-sticker on emotion
	progressTracker *ProgressTracker                     // progress trace for IM
	onMessage       func(channelType string)            // called on incoming message
	onSend          func(channelType string, err error) // called on outgoing reply
}

// NewRegistry creates a channel registry.
func NewRegistry() *Registry {
	return &Registry{channels: make(map[string]Channel)}
}

// SetMetricsHooks sets optional callbacks for recording channel message metrics.
func (r *Registry) SetMetricsHooks(onMessage func(string), onSend func(string, error)) {
	r.onMessage = onMessage
	r.onSend = onSend
}

// SetCommandInterceptor attaches a universal command interceptor that
// processes slash commands before they reach the planner.
func (r *Registry) SetCommandInterceptor(ci *CommandInterceptor) {
	r.interceptor = ci
}

// SetStickerEnricher attaches a sticker enricher that automatically
// appends stickers to replies based on detected emotion.
func (r *Registry) SetStickerEnricher(se *StickerEnricher) {
	r.enricher = se
}

// Register adds a channel.
func (r *Registry) Register(ch Channel) {
	r.channels[ch.Type()] = ch
}

// GetInterceptor returns the current CommandInterceptor (may be nil).
func (r *Registry) GetInterceptor() *CommandInterceptor { return r.interceptor }

// SetProgressTracker attaches a progress tracker for IM channels.
func (r *Registry) SetProgressTracker(pt *ProgressTracker) { r.progressTracker = pt }

// Get returns a channel by type.
func (r *Registry) Get(typ string) (Channel, bool) {
	ch, ok := r.channels[typ]
	return ch, ok
}

// All returns all registered channels.
func (r *Registry) All() []Channel {
	out := make([]Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		out = append(out, ch)
	}
	return out
}

// StartAll starts all registered channels concurrently.
// If a CommandInterceptor is set, slash commands are processed before reaching the planner.
// If metrics hooks are set, incoming and outgoing messages are recorded automatically.
func (r *Registry) StartAll(ctx context.Context, handler func(Message) Reply) {
	wrapped := handler

	// Layer 1: sticker enricher (enriches reply after planner runs)
	if r.enricher != nil {
		wrapped = r.enricher.Wrap(wrapped)
	}

	// Layer 2: command interceptor (intercepts slash commands before planner)
	if r.interceptor != nil {
		wrapped = r.interceptor.Wrap(wrapped)
	}

	// Layer 3: metrics hooks
	if r.onMessage != nil || r.onSend != nil {
		inner := wrapped
		wrapped = func(msg Message) Reply {
			if r.onMessage != nil {
				r.onMessage(msg.ChannelType)
			}
			reply := inner(msg)
			if r.onSend != nil {
				r.onSend(msg.ChannelType, nil)
			}
			return reply
		}
	}
	for _, ch := range r.channels {
		go func(c Channel) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("channel panic — recovered",
						"channel", c.Type(), "panic", rec,
						"stack", string(debugStack()))
					writePanicFile("channel."+c.Type(), rec)
				}
			}()
			if err := c.Start(ctx, wrapped); err != nil {
				slog.Error("channel stopped", "channel", c.Type(), "err", err)
			}
		}(ch)
	}
}

// ListGroups returns all groups for a specific channel, or all channels if typ is empty.
// It tries GroupLister first, then falls back to the GroupTracker.
func (r *Registry) ListGroups(ctx context.Context, typ string) ([]GroupInfo, error) {
	var result []GroupInfo

	iterate := func(ch Channel) {
		if lister, ok := ch.(GroupLister); ok {
			groups, err := lister.ListGroups(ctx)
			if err != nil {
				slog.Warn("channel ListGroups error", "channel", ch.Type(), "err", err)
			} else {
				result = append(result, groups...)
				return
			}
		}
		// Fallback to tracker
		if r.tracker != nil {
			result = append(result, r.tracker.Groups(ch.Type())...)
		}
	}

	if typ != "" {
		ch, ok := r.channels[typ]
		if !ok {
			return nil, nil
		}
		iterate(ch)
	} else {
		for _, ch := range r.channels {
			iterate(ch)
		}
	}
	return result, nil
}

// SetGroupTracker attaches a GroupTracker for message-based group discovery.
func (r *Registry) SetGroupTracker(t *GroupTracker) { r.tracker = t }

// GroupTracker returns the attached tracker (may be nil).
func (r *Registry) GetGroupTracker() *GroupTracker { return r.tracker }

// ──────────────────────────────────────────────
// GroupTracker — file-backed group discovery
// ──────────────────────────────────────────────

// GroupTracker records groups the bot has interacted with.
// Used for channels that lack a native "list groups" API (Telegram, DingTalk, etc.).
type GroupTracker struct {
	mu     sync.RWMutex
	groups map[string]GroupInfo // key = channelType + ":" + groupID
	path   string              // persistence path (e.g. data/groups.json)
	kvs    *iledger.KVConfigStore
}

// NewGroupTracker creates a new tracker that persists to the given file path.
func NewGroupTracker(path string) *GroupTracker {
	t := &GroupTracker{
		groups: make(map[string]GroupInfo),
		path:   path,
	}
	t.load()
	return t
}

// SetKVStore enables Ledger KV-backed persistence.
func (t *GroupTracker) SetKVStore(kvs *iledger.KVConfigStore) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.kvs = kvs
	t.loadFromKV()
}

// Track records a group. Call this when a group message is received.
func (t *GroupTracker) Track(g GroupInfo) {
	key := g.ChannelType + ":" + g.ID
	t.mu.Lock()
	existing, ok := t.groups[key]
	g.LastActive = time.Now().Format(time.RFC3339)
	if ok && g.Name == "" {
		g.Name = existing.Name
	}
	t.groups[key] = g
	t.mu.Unlock()
	t.save()
}

// Groups returns all tracked groups for a channel type (or all if typ is empty).
func (t *GroupTracker) Groups(typ string) []GroupInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []GroupInfo
	for _, g := range t.groups {
		if typ == "" || g.ChannelType == typ {
			out = append(out, g)
		}
	}
	return out
}

func (t *GroupTracker) load() {
	data, err := os.ReadFile(t.path)
	if err != nil {
		return
	}
	var groups []GroupInfo
	if err := json.Unmarshal(data, &groups); err != nil {
		slog.Warn("group tracker: parse error", "path", t.path, "err", err)
		return
	}
	t.mu.Lock()
	for _, g := range groups {
		t.groups[g.ChannelType+":"+g.ID] = g
	}
	t.mu.Unlock()
}

func (t *GroupTracker) loadFromKV() {
	if t.kvs == nil {
		return
	}
	var groups []GroupInfo
	found, err := t.kvs.Get(context.Background(), "groups", &groups)
	if err != nil {
		slog.Warn("group tracker: kv load failed", "err", err)
		return
	}
	if found {
		for _, g := range groups {
			t.groups[g.ChannelType+":"+g.ID] = g
		}
		slog.Info("group tracker: loaded from Ledger KV", "groups", len(groups))
	}
}

func (t *GroupTracker) save() {
	t.mu.RLock()
	groups := make([]GroupInfo, 0, len(t.groups))
	for _, g := range t.groups {
		groups = append(groups, g)
	}
	kvs := t.kvs
	t.mu.RUnlock()

	if kvs != nil {
		if err := kvs.Put(context.Background(), "groups", groups); err != nil {
			slog.Warn("group tracker: kv save failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(t.path, data, 0644); err != nil {
		slog.Warn("group tracker: save error", "path", t.path, "err", err)
	}
}

func debugStack() []byte {
	return rdebug.Stack()
}

func writePanicFile(label string, panicVal interface{}) {
	f, err := os.OpenFile("data/panic.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	stack := string(rdebug.Stack())
	entry := fmt.Sprintf(
		"=== PANIC [%s] at %s ===\npanic: %v\n%s\n\n",
		label, time.Now().Format(time.RFC3339), panicVal, stack,
	)
	f.WriteString(entry)
}
