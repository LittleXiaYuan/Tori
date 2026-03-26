package channel

import (
	"context"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// ThrottledProgressSender — debounce + summarize for IM edits
//
// Wraps a ProgressSender with:
//   - Debounce: 500ms min interval between EditMessage calls
//   - Rolling: keep only the last N lines visible
//   - Truncate: tool results > 200 chars are trimmed
//
// This prevents API rate-limiting (Telegram: 30 edits/sec global)
// and keeps the thinking message readable.
// ──────────────────────────────────────────────

// ThrottledProgressSender wraps a ProgressSender with debouncing and summarization.
type ThrottledProgressSender struct {
	inner    ProgressSender
	interval time.Duration // min interval between edits (default 500ms)
	maxLines int           // max lines shown (default 5)

	mu        sync.Mutex
	lines     []string
	messageID string
	target    string
	dirty     bool
	timer     *time.Timer
	ctx       context.Context
}

// NewThrottledProgressSender wraps a ProgressSender with debounce and summary.
func NewThrottledProgressSender(inner ProgressSender, ctx context.Context) *ThrottledProgressSender {
	return &ThrottledProgressSender{
		inner:    inner,
		interval: 500 * time.Millisecond,
		maxLines: 5,
		ctx:      ctx,
	}
}

// SendAndGetID delegates to the inner sender.
func (t *ThrottledProgressSender) SendAndGetID(ctx context.Context, target string, reply Reply) (string, error) {
	id, err := t.inner.SendAndGetID(ctx, target, reply)
	if err == nil {
		t.mu.Lock()
		t.messageID = id
		t.target = target
		t.mu.Unlock()
	}
	return id, err
}

// EditMessage is NOT called directly — use AppendLine instead.
func (t *ThrottledProgressSender) EditMessage(ctx context.Context, target string, messageID string, content string) error {
	return t.inner.EditMessage(ctx, target, messageID, content)
}

// AppendLine adds a progress line and schedules a debounced edit.
func (t *ThrottledProgressSender) AppendLine(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lines = append(t.lines, line)
	t.dirty = true

	if t.timer == nil {
		t.timer = time.AfterFunc(t.interval, t.flush)
	}
}

// Flush forces an immediate edit (call when planner is done).
func (t *ThrottledProgressSender) Flush() {
	t.mu.Lock()
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	t.mu.Unlock()
	t.flush()
}

func (t *ThrottledProgressSender) flush() {
	t.mu.Lock()
	if !t.dirty || t.messageID == "" {
		t.mu.Unlock()
		return
	}
	t.dirty = false
	t.timer = nil

	// Build display text with rolling window
	total := len(t.lines)
	var sb strings.Builder

	if total > t.maxLines {
		sb.WriteString("⋯ (")
		sb.WriteString(itoa(total - t.maxLines))
		sb.WriteString(" 步已完成)\n")
	}

	start := 0
	if total > t.maxLines {
		start = total - t.maxLines
	}
	for _, l := range t.lines[start:] {
		sb.WriteString(l)
		sb.WriteString("\n")
	}

	text := sb.String()
	msgID := t.messageID
	target := t.target
	t.mu.Unlock()

	_ = t.inner.EditMessage(t.ctx, target, msgID, text)
}

// TruncateResult truncates a tool result string for display.
func TruncateResult(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
