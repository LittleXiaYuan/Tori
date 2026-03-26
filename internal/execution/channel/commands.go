package channel

import (
	"strings"
)

// CommandHandler processes a slash command and returns (reply, handled).
// If handled is true, the command was recognized and the reply should be sent
// instead of forwarding the message to the planner.
type CommandHandler func(msg Message, command string, args string) (Reply, bool)

// CommandInterceptor is a middleware that intercepts slash commands from all
// channels before they reach the planner. This enables universal command
// support (/add, /sticker, /sticker-del, etc.) across all 13+ IM platforms.
type CommandInterceptor struct {
	handlers []CommandHandler
}

// NewCommandInterceptor creates a new interceptor with no handlers.
func NewCommandInterceptor() *CommandInterceptor {
	return &CommandInterceptor{}
}

// Register adds a command handler. Handlers are tried in order;
// the first one that returns handled=true wins.
func (ci *CommandInterceptor) Register(h CommandHandler) {
	ci.handlers = append(ci.handlers, h)
}

// Intercept checks if a message is a slash command and tries all handlers.
// Returns (reply, true) if a handler processed the command.
// Returns (Reply{}, false) if no handler matched or the message isn't a command.
func (ci *CommandInterceptor) Intercept(msg Message) (Reply, bool) {
	if len(ci.handlers) == 0 {
		return Reply{}, false
	}

	cmd, args := parseSlashCommand(msg.Content)
	if cmd == "" {
		return Reply{}, false
	}

	for _, h := range ci.handlers {
		if reply, ok := h(msg, cmd, args); ok {
			return reply, true
		}
	}
	return Reply{}, false
}

// Wrap returns a wrapped handler that intercepts commands before delegating.
func (ci *CommandInterceptor) Wrap(next func(Message) Reply) func(Message) Reply {
	return func(msg Message) Reply {
		if reply, ok := ci.Intercept(msg); ok {
			return reply
		}
		return next(msg)
	}
}

// parseSlashCommand extracts the command and arguments from a message.
// "/sticker happy" → ("/sticker", "happy")
// "hello world" → ("", "")
func parseSlashCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if len(text) == 0 || text[0] != '/' {
		return "", ""
	}

	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])

	// Strip bot mention: /sticker@mybot → /sticker
	if at := strings.IndexByte(cmd, '@'); at > 0 {
		cmd = cmd[:at]
	}

	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return cmd, args
}
