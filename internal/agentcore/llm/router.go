package llm

import (
	"context"
	"strings"
	"sync"
)

// ModelRoute defines when to use a specific model.
type ModelRoute struct {
	Model      string   // model identifier
	Categories []string // task categories: "code", "chat", "analysis", "translation", "embedding"
	MaxTokens  int      // max tokens for this model
	Priority   int      // higher = preferred
}

// Router selects the best model based on task type.
type Router struct {
	mu      sync.RWMutex
	routes  []ModelRoute
	client  *Client
	default_ string
}

// NewRouter creates a model router with a default model.
func NewRouter(client *Client) *Router {
	return &Router{client: client, default_: client.model}
}

// AddRoute registers a model route.
func (r *Router) AddRoute(route ModelRoute) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = append(r.routes, route)
}

// SelectModel picks the best model for a given task category.
func (r *Router) SelectModel(category string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	category = strings.ToLower(category)
	var best *ModelRoute
	for i := range r.routes {
		rt := &r.routes[i]
		for _, cat := range rt.Categories {
			if strings.ToLower(cat) == category {
				if best == nil || rt.Priority > best.Priority {
					best = rt
				}
			}
		}
	}
	if best != nil {
		return best.Model
	}
	return r.default_
}

// Chat routes the request to the best model for the detected category.
func (r *Router) Chat(ctx context.Context, messages []Message, temperature float64, category string) (string, error) {
	model := r.SelectModel(category)
	return r.client.ChatWithModel(ctx, model, messages, temperature)
}

// DetectCategory uses simple heuristics to categorize the user message.
func DetectCategory(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case containsAny(lower, []string{"代码", "code", "函数", "function", "编程", "debug", "bug", "实现", "implement"}):
		return "code"
	case containsAny(lower, []string{"翻译", "translate", "英文", "中文", "japanese"}):
		return "translation"
	case containsAny(lower, []string{"分析", "analyze", "统计", "数据", "报告", "report"}):
		return "analysis"
	case containsAny(lower, []string{"总结", "summarize", "摘要", "概括"}):
		return "analysis"
	default:
		return "chat"
	}
}

// Routes returns all registered routes.
func (r *Router) Routes() []ModelRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ModelRoute, len(r.routes))
	copy(out, r.routes)
	return out
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
