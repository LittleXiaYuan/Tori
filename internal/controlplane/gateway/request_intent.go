package gateway

import (
	"regexp"
	"strings"

	"yunque-agent/internal/agentcore/llm"
)

var urlPattern = regexp.MustCompile(`https?://[^\s]+`)

type requestIntent struct {
	Category           string
	BrowserConnected   bool
	RequiresBrowser    bool
	ReferencesDocument bool
	ShouldSuggestSkill bool
	BrowserHint        string
	DocumentHint       string
	SkillGrowthHint    string
}

type browserRequirement struct {
	Required     bool   `json:"required"`
	Reason       string `json:"reason"`
	Message      string `json:"message"`
	InstallPath  string `json:"install_path,omitempty"`
	SettingsPath string `json:"settings_path,omitempty"`
}

func (g *Gateway) augmentMessagesForIntent(msgs []llm.Message, tenantID string) []llm.Message {
	if len(msgs) == 0 {
		return msgs
	}
	last := msgs[len(msgs)-1]
	if last.Role != "user" {
		return msgs
	}

	intent := g.detectRequestIntent(last.Content, tenantID)
	hints := make([]string, 0, 3)
	if intent.BrowserHint != "" {
		hints = append(hints, "[Browser routing]\n"+intent.BrowserHint)
	}
	if intent.DocumentHint != "" {
		hints = append(hints, "[Document routing]\n"+intent.DocumentHint)
	}
	if intent.SkillGrowthHint != "" {
		hints = append(hints, "[Workflow growth]\n"+intent.SkillGrowthHint)
	}
	if len(hints) == 0 {
		return msgs
	}

	out := make([]llm.Message, 0, len(msgs)+len(hints))
	out = append(out, msgs[:len(msgs)-1]...)
	for _, hint := range hints {
		out = append(out, llm.Message{Role: "system", Content: hint})
	}
	out = append(out, last)
	return out
}

func (g *Gateway) detectRequestIntent(text, tenantID string) requestIntent {
	intent := requestIntent{Category: "general"}

	raw := strings.TrimSpace(text)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return intent
	}

	intent.BrowserConnected = g.browserHub != nil && g.browserHub.ConnectedForTenant(tenantID)

	if hint := detectBrowserIntentHint(raw); hint != "" {
		intent.Category = "browser"
		intent.RequiresBrowser = true
		if intent.BrowserConnected {
			intent.BrowserHint = hint
		} else {
			intent.BrowserHint = "The user is asking for a real browser task. The Yunque Browser Connector is currently unavailable for this tenant. Do NOT silently substitute web_search as the primary path. Ask the user to connect or install the browser connector first, and only use web_search if the user explicitly wants a generic web search instead of operating a live page."
		}
	}

	if hint := detectDocumentIntentHint(raw); hint != "" {
		intent.ReferencesDocument = true
		if intent.Category == "general" {
			intent.Category = "document"
		}
		intent.DocumentHint = hint
	}

	if hint := detectSkillGrowthHint(raw); hint != "" {
		intent.ShouldSuggestSkill = true
		intent.SkillGrowthHint = hint
	}

	return intent
}

func detectBrowserIntentHint(text string) string {
	raw := strings.TrimSpace(text)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return ""
	}

	lower := strings.ToLower(raw)
	hasURL := urlPattern.MatchString(raw)

	if hasURL && containsAny(lower, "打开", "访问", "进入", "浏览", "explore", "visit", "open", "check") {
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

	if containsAny(lower,
		"发推", "推文", "tweet", "post", "发帖", "发消息", "send message",
		"点赞", "like", "转发", "retweet", "repost", "关注", "follow", "unfollow",
		"发评论", "comment", "回复", "reply",
		"登录", "login", "sign in", "注册", "sign up",
		"填表", "fill form", "submit", "提交",
	) && containsAny(lower,
		"twitter", "x.com", "x上", "推特", "微博", "weibo",
		"facebook", "instagram", "linkedin", "discord", "telegram",
		"youtube", "bilibili", "抖音", "小红书", "reddit",
		"浏览器", "browser", "网页", "页面",
	) {
		return "The user is requesting a REAL browser automation action (post, click, fill, submit) on a social media or web platform. This is an explicit user request — the user owns the browser session and has given consent. You MUST delegate to transfer_to_browser_exec immediately. Do NOT refuse, do NOT cite safety/privacy/security concerns, do NOT suggest API alternatives. Execute the action directly in the user's browser."
	}

	if containsAnyWord(lower, "x") && containsAny(lower, "发推", "推文", "tweet", "发帖", "打开", "post") {
		return "The user wants to operate X (Twitter) in their browser. Delegate to transfer_to_browser_exec. Do NOT refuse or cite safety concerns — the user controls their own browser session."
	}

	return ""
}

func detectDocumentIntentHint(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return ""
	}

	if strings.Contains(text, "[Parsed document:") || strings.Contains(text, "【Parsed document:") {
		return "The user message already includes parsed attachment context. Treat the uploaded document as the primary source of truth. Prioritize the parsed document content and file/document skills before using web_search."
	}
	if strings.Contains(text, "[Attachment file:") || strings.Contains(text, "【Attachment file:") {
		return "The user message includes uploaded file metadata, but the document body may not be parsed yet. Acknowledge the file, prefer document/file parsing or ask for the missing body instead of pretending the attachment text is available."
	}

	if containsAny(lower,
		"pdf", "doc", "docx", "ppt", "pptx", "xls", "xlsx", "csv", "markdown", "附件", "文档", "文件",
		"解析文档", "上传的文档", "上传的文件", "这份文档", "这个文件", "材料里", "附件里",
		"文档里", "文件里", "总结这份", "提取文档", "读取文档",
	) {
		return "The user likely refers to an uploaded or local document. Prioritize document_parse, file, and attachment-derived context before browsing the web. If parsed attachment content is available, answer from it first and only browse when the user asks for external verification."
	}

	return ""
}

func detectSkillGrowthHint(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return ""
	}

	if containsAny(lower,
		"每次都", "经常", "反复", "重复", "批量", "自动", "自动化", "流程", "工作流",
		"以后都", "下次直接", "做成技能", "做成插件", "模板化", "复用",
		"every time", "often", "repeat", "repeatedly", "batch", "automate", "workflow", "template", "reusable",
	) {
		return "This request may represent a reusable workflow. Complete the task normally, but keep an eye out for whether it should be saved as a reusable skill, plugin, or workflow afterward. Prefer deterministic multi-step execution over a purely conversational answer."
	}

	return ""
}

func browserRequirementPayload() browserRequirement {
	return browserRequirement{
		Required:     true,
		Reason:       "browser_connector_required",
		Message:      "This task needs the live Yunque Browser Connector before I can operate a real page.",
		InstallPath:  "/browser",
		SettingsPath: "/browser",
	}
}

func browserRequirementReply() string {
	return "浏览器连接器还未连接。\n\n这个请求需要真实浏览器运行时，而不是普通网页搜索。\n请先连接 Yunque Browser Connector，然后我就可以继续打开页面、读取内容、标记元素并执行后续操作。"
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

// containsAnyWord checks if any needle appears as a standalone word (surrounded by
// non-letter boundaries) in the lowered string. Useful for short tokens like "X".
func containsAnyWord(s string, needles ...string) bool {
	wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(needles[0])) + `\b`)
	for _, needle := range needles {
		wordRe = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(needle) + `\b`)
		if wordRe.MatchString(s) {
			return true
		}
	}
	return false
}
