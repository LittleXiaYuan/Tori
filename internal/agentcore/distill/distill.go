package distill

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// LLMFunc abstracts an LLM call (typically Fast tier).
type LLMFunc func(ctx context.Context, system, user string) (string, error)

// StoreFunc writes a distilled rule to memory.
type StoreFunc func(ctx context.Context, key, value, category string) error

// SearchFunc checks if a distilled rule already exists for a query.
type SearchFunc func(ctx context.Context, query string) (string, bool)

// Distiller compresses Expert model outputs into reusable rules.
type Distiller struct {
	mu          sync.Mutex
	llmCall     LLMFunc
	store       StoreFunc
	search      SearchFunc
	minReplyLen int // only distill replies longer than this
	cooldown    map[string]time.Time
}

// New creates a knowledge distiller.
func New(llm LLMFunc) *Distiller {
	return &Distiller{
		llmCall:     llm,
		minReplyLen: 200, // only distill substantial answers
		cooldown:    make(map[string]time.Time),
	}
}

// SetStore attaches the memory storage callback.
func (d *Distiller) SetStore(fn StoreFunc) { d.store = fn }

// SetSearch attaches the memory search callback (for dedup).
func (d *Distiller) SetSearch(fn SearchFunc) { d.search = fn }

// ShouldDistill checks if a response from a given model tier should be distilled.
func (d *Distiller) ShouldDistill(tier, reply string) bool {
	if tier != "expert" {
		return false
	}
	if len([]rune(reply)) < d.minReplyLen {
		return false
	}
	return true
}

// CheckCache returns a cached distilled rule if one matches the query.
// Returns the rule and true if found, empty and false otherwise.
func (d *Distiller) CheckCache(ctx context.Context, query string) (string, bool) {
	if d.search == nil {
		return "", false
	}
	return d.search(ctx, query)
}

// Distill compresses an expert answer into a reusable rule and stores it.
// This runs asynchronously and should not block the main request path.
func (d *Distiller) Distill(ctx context.Context, question, expertReply string) {
	if d.llmCall == nil || d.store == nil {
		return
	}

	// Dedup: skip if we recently distilled for a similar question
	d.mu.Lock()
	key := normalizeKey(question)
	if last, ok := d.cooldown[key]; ok && time.Since(last) < 30*time.Minute {
		d.mu.Unlock()
		return
	}
	d.cooldown[key] = time.Now()
	d.mu.Unlock()

	go d.distillAsync(ctx, question, expertReply, key)
}

func (d *Distiller) distillAsync(ctx context.Context, question, expertReply, key string) {
	system := "你是知识蒸馏引擎。将以下专家回答提炼为一条可复用的规则。\n" +
		"格式：当[X]时，应该[Y]\n" +
		"要求：一句话，不超过100字，包含关键知识点。只输出规则，不要其他文字。"
	user := fmt.Sprintf("问题：%s\n\n专家回答：%s", truncate(question, 500), truncate(expertReply, 2000))

	rule, err := d.llmCall(ctx, system, user)
	if err != nil {
		slog.Warn("distill: LLM call failed", "err", err)
		return
	}
	rule = strings.TrimSpace(rule)
	if rule == "" || len([]rune(rule)) > 200 {
		return
	}

	// Classify the category
	category := classifyCategory(question)

	if err := d.store(ctx, key, rule, category); err != nil {
		slog.Warn("distill: store failed", "err", err)
		return
	}
	slog.Info("distill: rule stored", "key", key, "category", category, "rule_len", len([]rune(rule)))
}

// CleanCooldown removes stale cooldown entries.
func (d *Distiller) CleanCooldown() {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	for k, t := range d.cooldown {
		if now.Sub(t) > 1*time.Hour {
			delete(d.cooldown, k)
		}
	}
}

// ── Helpers ──

func normalizeKey(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > 80 {
		r = r[:80]
	}
	return string(r)
}

func classifyCategory(question string) string {
	q := strings.ToLower(question)
	switch {
	case strings.Contains(q, "代码") || strings.Contains(q, "code") || strings.Contains(q, "bug"):
		return "coding"
	case strings.Contains(q, "部署") || strings.Contains(q, "deploy") || strings.Contains(q, "docker"):
		return "devops"
	case strings.Contains(q, "安全") || strings.Contains(q, "security"):
		return "security"
	case strings.Contains(q, "数据") || strings.Contains(q, "database") || strings.Contains(q, "sql"):
		return "data"
	default:
		return "general"
	}
}

func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}
