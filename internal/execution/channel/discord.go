package channel

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const discordMaxLength = 2000

// Discord implements the Channel interface for Discord Bot API.
type Discord struct {
	token        string
	session      *discordgo.Session
	mu           sync.Mutex
	seenMessages map[string]time.Time
}

// NewDiscord creates a Discord channel with the given bot token.
func NewDiscord(token string) *Discord {
	return &Discord{
		token:        token,
		seenMessages: make(map[string]time.Time),
	}
}

func (d *Discord) Type() string { return "discord" }

func (d *Discord) Start(ctx context.Context, handler func(Message) Reply) error {
	session, err := discordgo.New("Bot " + d.token)
	if err != nil {
		return fmt.Errorf("discord create session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	d.mu.Lock()
	d.session = session
	d.mu.Unlock()

	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}
		if ctx.Err() != nil {
			return
		}
		if d.isDuplicate(m.ID) {
			return
		}

		text := strings.TrimSpace(m.Content)
		if text == "" {
			return
		}

		// Determine chat type
		chatType := "direct"
		if m.GuildID != "" {
			chatType = "group"
		}

		// Check if bot is mentioned
		botID := s.State.User.ID
		isMentioned := false
		for _, mention := range m.Mentions {
			if mention != nil && mention.ID == botID {
				isMentioned = true
				break
			}
		}
		isReplyToBot := m.ReferencedMessage != nil &&
			m.ReferencedMessage.Author != nil &&
			m.ReferencedMessage.Author.ID == botID

		// In group chats, only respond when mentioned or replied to
		if chatType == "group" && !isMentioned && !isReplyToBot {
			return
		}

		// Strip bot mention from text
		text = stripMention(text, botID)

		msg := Message{
			ChannelType: "discord",
			ChannelID:   m.ChannelID,
			UserID:      m.Author.ID,
			UserName:    m.Author.Username,
			Content:     text,
			Extra: map[string]string{
				"message_id": m.ID,
				"chat_type":  chatType,
				"guild_id":   m.GuildID,
			},
		}

		slog.Info("discord message received",
			"user", m.Author.Username,
			"channel", m.ChannelID,
			"type", chatType,
		)

		go func() {
			// Show typing indicator
			_ = s.ChannelTyping(m.ChannelID)

			reply := handler(msg)
			if err := d.Send(ctx, m.ChannelID, reply); err != nil {
				slog.Error("discord send reply failed", "err", err)
			}
		}()
	})

	if err := session.Open(); err != nil {
		return fmt.Errorf("discord open: %w", err)
	}
	slog.Info("discord bot connected", "user", session.State.User.Username)

	<-ctx.Done()
	slog.Info("discord bot shutting down")
	return session.Close()
}

func (d *Discord) Send(_ context.Context, target string, reply Reply) error {
	d.mu.Lock()
	session := d.session
	d.mu.Unlock()

	if session == nil {
		return fmt.Errorf("discord session not initialized")
	}

	content := reply.Content
	if content == "" {
		return nil
	}

	// Split long messages
	chunks := splitDiscordMessage(content)
	for _, chunk := range chunks {
		_, err := session.ChannelMessageSend(target, chunk)
		if err != nil {
			return fmt.Errorf("discord send: %w", err)
		}
	}
	return nil
}

// isDuplicate checks and records message IDs to prevent duplicate processing.
func (d *Discord) isDuplicate(messageID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	// Clean expired entries
	for k, t := range d.seenMessages {
		if now.Sub(t) > time.Minute {
			delete(d.seenMessages, k)
		}
	}

	if _, ok := d.seenMessages[messageID]; ok {
		return true
	}
	d.seenMessages[messageID] = now
	return false
}

// stripMention removes bot mention tags from message text.
func stripMention(text, botID string) string {
	text = strings.ReplaceAll(text, "<@"+botID+">", "")
	text = strings.ReplaceAll(text, "<@!"+botID+">", "")
	return strings.TrimSpace(text)
}

// splitDiscordMessage splits a message into chunks that fit Discord's 2000 char limit.
func splitDiscordMessage(text string) []string {
	return SplitMessage(text, discordMaxLength)
}

// ListGroups returns all guilds (servers) the Discord bot is currently in.
func (d *Discord) ListGroups(_ context.Context) ([]GroupInfo, error) {
	d.mu.Lock()
	session := d.session
	d.mu.Unlock()

	if session == nil {
		return nil, fmt.Errorf("discord session not initialized")
	}

	guilds, err := session.UserGuilds(100, "", "", false)
	if err != nil {
		return nil, fmt.Errorf("discord list guilds: %w", err)
	}

	out := make([]GroupInfo, 0, len(guilds))
	for _, g := range guilds {
		out = append(out, GroupInfo{
			ID:          g.ID,
			Name:        g.Name,
			ChannelType: "discord",
			ChatType:    "guild",
			MemberCount: g.ApproximateMemberCount,
		})
	}
	return out, nil
}

// React adds an emoji reaction to a Discord message.
// emoji should be a unicode emoji (e.g. "👍") or a custom emoji in the format "name:id".
// Pass empty emoji to remove the bot's reaction.
func (d *Discord) React(_ context.Context, target string, messageID string, emoji string) error {
	d.mu.Lock()
	session := d.session
	d.mu.Unlock()

	if session == nil {
		return fmt.Errorf("discord session not initialized")
	}

	if emoji == "" {
		// Remove all reactions from the message
		return session.MessageReactionsRemoveAll(target, messageID)
	}

	return session.MessageReactionAdd(target, messageID, emoji)
}

// Ensure Discord implements optional interfaces
var (
	_ Channel = (*Discord)(nil)
	_ Reactor = (*Discord)(nil)
)
