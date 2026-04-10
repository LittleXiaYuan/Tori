package browserskill

// Scenario is a preset browser automation test scenario.
type Scenario struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Icon        string           `json:"icon"`
	Steps       []map[string]any `json:"steps"`
}

// PresetScenarios returns all built-in browser test scenarios.
func PresetScenarios() []Scenario {
	return []Scenario{
		{
			ID: "google-search", Name: "Google 搜索", Icon: "🔍",
			Description: "在 Google 首页搜索关键词并浏览结果",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://www.google.com"},
				{"type": "browser_input", "target": map[string]any{"strategy": "bySelector", "selector": `textarea[name="q"]`}, "text": "Yunque AI Agent", "pressEnter": true},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "baidu-search", Name: "百度搜索", Icon: "🔍",
			Description: "在百度首页搜索关键词并浏览结果",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://www.baidu.com"},
				{"type": "browser_input", "target": map[string]any{"strategy": "bySelector", "selector": "#kw"}, "text": "人工智能", "pressEnter": true},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "youtube-search", Name: "YouTube 搜索", Icon: "▶️",
			Description: "在 YouTube 搜索视频并浏览结果",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://www.youtube.com"},
				{"type": "browser_input", "target": map[string]any{"strategy": "bySelector", "selector": "input#search"}, "text": "lofi hip hop", "pressEnter": true},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "duckduckgo-search", Name: "DuckDuckGo 搜索", Icon: "🦆",
			Description: "在 DuckDuckGo 搜索引擎搜索关键词",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://duckduckgo.com"},
				{"type": "browser_input", "target": map[string]any{"strategy": "bySelector", "selector": `input[name="q"]`}, "text": "Yunque AI", "pressEnter": true},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "linkedin-messaging", Name: "LinkedIn 消息", Icon: "💼",
			Description: "导航到 LinkedIn 消息页面并发送一条问候消息",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://www.linkedin.com/messaging"},
				{"type": "browser_screenshot"},
				{"type": "browser_click", "target": map[string]any{"strategy": "bySelector", "selector": ".msg-form__contenteditable"}},
				{"type": "browser_input", "text": "Hi! This is a test message from Yunque Agent."},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "twitter-post", Name: "Twitter/X 发帖", Icon: "🐦",
			Description: "导航到 Twitter 首页并输入一条推文",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://x.com/home"},
				{"type": "browser_screenshot"},
				{"type": "browser_click", "target": map[string]any{"strategy": "bySelector", "selector": `[data-testid="tweetTextarea_0"]`}},
				{"type": "browser_input", "text": "Hello World from Yunque Agent! 🐦"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "gmail-scroll", Name: "Gmail 收件箱", Icon: "📧",
			Description: "打开 Gmail 收件箱并滚动浏览邮件",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://mail.google.com"},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "bing-search", Name: "Bing 搜索", Icon: "🔎",
			Description: "在 Bing 搜索引擎搜索关键词",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://www.bing.com"},
				{"type": "browser_input", "target": map[string]any{"strategy": "bySelector", "selector": "#sb_form_q"}, "text": "AI Agent 2025", "pressEnter": true},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "wikipedia-browse", Name: "Wikipedia 浏览", Icon: "📚",
			Description: "在 Wikipedia 搜索并浏览一篇文章",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://en.wikipedia.org"},
				{"type": "browser_input", "target": map[string]any{"strategy": "bySelector", "selector": "#searchInput"}, "text": "Artificial intelligence", "pressEnter": true},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_get_content"},
			},
		},
		{
			ID: "github-browse", Name: "GitHub 浏览", Icon: "🐙",
			Description: "导航到 GitHub 并搜索项目",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://github.com"},
				{"type": "browser_input", "target": map[string]any{"strategy": "bySelector", "selector": `input[name="q"]`}, "text": "AI agent framework", "pressEnter": true},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "scroll-stress-test", Name: "滚动压力测试", Icon: "🔄",
			Description: "连续滚动到底部和顶部，测试滚动功能稳定性",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)"},
				{"type": "browser_scroll", "direction": "down", "to_end": true},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "up", "to_end": true},
				{"type": "browser_screenshot"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_scroll", "direction": "down"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "keyboard-test", Name: "键盘按键测试", Icon: "⌨️",
			Description: "测试各种键盘按键和组合键",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://www.google.com"},
				{"type": "browser_click", "target": map[string]any{"strategy": "bySelector", "selector": `textarea[name="q"]`}},
				{"type": "browser_press_key", "key": "a"},
				{"type": "browser_press_key", "key": "b"},
				{"type": "browser_press_key", "key": "c"},
				{"type": "browser_press_key", "key": "Control+a"},
				{"type": "browser_press_key", "key": "Backspace"},
				{"type": "browser_input", "text": "Key test complete"},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "example-click", Name: "链接点击测试", Icon: "🔗",
			Description: "在 example.com 点击链接验证点击功能",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://example.com"},
				{"type": "browser_screenshot"},
				{"type": "browser_click", "target": map[string]any{"strategy": "bySelector", "selector": "a"}},
				{"type": "browser_screenshot"},
			},
		},
		{
			ID: "content-extraction", Name: "内容提取测试", Icon: "📄",
			Description: "导航到 Hacker News 并提取页面文本内容",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://news.ycombinator.com"},
				{"type": "browser_screenshot"},
				{"type": "browser_get_content"},
				{"type": "browser_scroll", "direction": "down", "to_end": true},
				{"type": "browser_get_content"},
			},
		},
		{
			ID: "user-takeover", Name: "用户接管测试", Icon: "🤝",
			Description: "测试 AI 暂停并让用户接管浏览器的流程",
			Steps: []map[string]any{
				{"type": "browser_navigate", "url": "https://example.com"},
				{"type": "browser_screenshot"},
				{"type": "session_status", "status": "paused", "sessionTitle": "等待用户操作"},
			},
		},
	}
}
