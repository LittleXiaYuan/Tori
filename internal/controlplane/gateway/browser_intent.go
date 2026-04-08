package gateway

import (
	"regexp"
	"strings"

	"yunque-agent/internal/agentcore/llm"
)

var urlPattern = regexp.MustCompile(`https?://[^\s]+`)

func (g *Gateway) augmentMessagesForBrowserIntent(msgs []llm.Message, tenantID string) []llm.Message {
	if len(msgs) == 0 {
		return msgs
	}
	if g.browserHub == nil || !g.browserHub.ConnectedForTenant(tenantID) {
		return msgs
	}
	last := msgs[len(msgs)-1]
	if last.Role != "user" {
		return msgs
	}
	hint := detectBrowserIntentHint(last.Content)
	if hint == "" {
		return msgs
	}

	out := make([]llm.Message, 0, len(msgs)+1)
	out = append(out, msgs[:len(msgs)-1]...)
	out = append(out, llm.Message{
		Role: "system",
		Content: "[Browser routing]\n" + hint,
	})
	out = append(out, last)
	return out
}

func detectBrowserIntentHint(text string) string {
	raw := strings.TrimSpace(text)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return ""
	}

	lower := strings.ToLower(raw)
	hasURL := urlPattern.MatchString(raw)

	if hasURL && containsAny(lower, "打开", "访问", "进入", "浏览", "看看", "看下", "explore", "visit", "open", "check") {
		return "The user explicitly wants to use the real browser on a provided URL. You MUST prioritize browser_navigate first, then continue with browser_get_content / browser_mark_elements / browser_click / browser_scroll as needed. Do not use web_search as the primary path when the URL is already known."
	}

	if containsAny(lower,
		"当前页面", "这个页面", "该页面", "当前网页", "这个网页", "网页内容", "页面内容",
		"提取页面", "提取网页", "总结页面", "分析页面", "读取页面", "读取网页",
		"mark elements", "标记元素", "编号元素", "给页面编号", "看看页面",
	) {
		return "The user is asking about the current webpage or wants page interaction. You MUST prioritize browser_get_content / browser_mark_elements / browser_get_elements / browser_click instead of web_search. Treat the browser as the primary runtime."
	}

	if hasURL && containsAny(lower, "总结", "分析", "推荐", "探索", "explore", "summarize", "analyze", "recommend", "彻底探索") {
		return "The user wants browser-based exploration of a known URL. Start with browser_navigate, then inspect the loaded page using browser_get_content or browser_get_elements before answering. Only fall back to web_search if the browser path is insufficient."
	}

	if containsAny(lower,
		"youtube", "github", "notion", "twitter", "x.com", "reddit", "bilibili", "抖音", "小红书",
	) && containsAny(lower, "打开", "访问", "看看", "看下", "进去", "浏览", "分析", "探索") {
		return "The request targets a real website or app experience. Prefer browser-based execution over web_search. Navigate in the browser first, then inspect the actual page state."
	}

	return ""
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
