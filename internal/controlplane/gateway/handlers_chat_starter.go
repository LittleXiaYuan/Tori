package gateway

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/apperror"
)

// starterSuggestionsTTL bounds how long a tenant's generated openers are reused.
// The empty chat screen mounts often (every new conversation, tab switch); a
// ~10min cache keeps the LLM cost negligible while staying fresh enough that
// new conversations start influencing suggestions within one short session.
const starterSuggestionsTTL = 10 * time.Minute

type starterCacheEntry struct {
	suggestions []planner.StarterSuggestion
	expiresAt   time.Time
}

// starterSuggestionCache is a tiny per-tenant TTL memo for generated openers.
// It is package-level on purpose: it is pure memoization with no lifecycle to
// manage, so it stays out of the Gateway god-object and its wiring.
var starterSuggestionCache = struct {
	sync.Mutex
	byTenant map[string]starterCacheEntry
}{byTenant: map[string]starterCacheEntry{}}

func starterCacheGet(tid string) ([]planner.StarterSuggestion, bool) {
	starterSuggestionCache.Lock()
	defer starterSuggestionCache.Unlock()
	e, ok := starterSuggestionCache.byTenant[tid]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.suggestions, true
}

func starterCachePut(tid string, suggestions []planner.StarterSuggestion) {
	starterSuggestionCache.Lock()
	defer starterSuggestionCache.Unlock()
	starterSuggestionCache.byTenant[tid] = starterCacheEntry{
		suggestions: suggestions,
		expiresAt:   time.Now().Add(starterSuggestionsTTL),
	}
}

// handleStarterSuggestions returns personalized empty-screen chat openers for
// the current tenant. On any failure (no planner, LLM error, empty result) it
// responds 200 with an empty list so the frontend cleanly falls back to its
// curated static set — this endpoint is best-effort decoration, never a hard
// dependency for starting a chat.
func (g *Gateway) handleStarterSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	tid := tenantFromCtx(r.Context())

	if cached, ok := starterCacheGet(tid); ok {
		writeJSON(w, map[string]any{"suggestions": cached, "cached": true})
		return
	}

	if g.planner == nil {
		writeJSON(w, map[string]any{"suggestions": []planner.StarterSuggestion{}})
		return
	}

	profile := g.buildStarterProfile(tid)
	suggestions, err := g.planner.GenerateStarterSuggestions(r.Context(), profile)
	if err != nil || len(suggestions) == 0 {
		writeJSON(w, map[string]any{"suggestions": []planner.StarterSuggestion{}})
		return
	}

	starterCachePut(tid, suggestions)
	writeJSON(w, map[string]any{"suggestions": suggestions})
}

// buildStarterProfile assembles the lightweight context string the model uses
// to personalize openers: time of day, the tenant's most recent conversation
// titles/summaries, and installed pack names.
func (g *Gateway) buildStarterProfile(tid string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "当前时段：%s\n", timeOfDayLabel(time.Now()))

	if g.convStore != nil {
		recent := recentConversations(g.convStore.ListByTenant(tid), 6)
		if len(recent) > 0 {
			b.WriteString("\n近期对话：\n")
			for _, s := range recent {
				title := strings.TrimSpace(s.Name)
				if title == "" {
					title = "（未命名）"
				}
				summary := strings.TrimSpace(s.Summary)
				if summary != "" {
					fmt.Fprintf(&b, "- %s：%s\n", title, clipRunesGw(summary, 80))
				} else {
					fmt.Fprintf(&b, "- %s\n", title)
				}
			}
		}
	}

	if g.packRegistry != nil {
		enabled := g.packRegistry.Enabled()
		if len(enabled) > 0 {
			names := make([]string, 0, len(enabled))
			for _, p := range enabled {
				name := strings.TrimSpace(p.Manifest.Name)
				if name == "" {
					name = p.Manifest.ID
				}
				if name != "" {
					names = append(names, name)
				}
				if len(names) >= 12 {
					break
				}
			}
			if len(names) > 0 {
				fmt.Fprintf(&b, "\n已安装能力：%s\n", strings.Join(names, "、"))
			}
		}
	}

	return b.String()
}

// recentConversations returns the n most-recently-updated non-archived sessions.
func recentConversations(sessions []session.Session, n int) []session.Session {
	filtered := sessions[:0:0]
	for _, s := range sessions {
		if s.ArchivedAt != nil {
			continue
		}
		filtered = append(filtered, s)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})
	if len(filtered) > n {
		filtered = filtered[:n]
	}
	return filtered
}

func timeOfDayLabel(t time.Time) string {
	switch h := t.Hour(); {
	case h < 6:
		return "凌晨"
	case h < 11:
		return "上午"
	case h < 13:
		return "中午"
	case h < 18:
		return "下午"
	default:
		return "晚上"
	}
}

func clipRunesGw(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
