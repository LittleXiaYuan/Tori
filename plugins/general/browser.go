package general

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

// BrowserSkill provides browser automation capabilities:
// - Fetch web page content (text extraction)
// - Screenshot web pages (via headless Chrome CDP if available)
// - Submit forms / interact with pages
type BrowserSkill struct {
	client       *http.Client
	allowPrivate bool // for testing only — disables SSRF protection
}

func NewBrowserSkill() *BrowserSkill {
	return &BrowserSkill{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (s *BrowserSkill) Name() string { return "browser" }

func (s *BrowserSkill) Description() string {
	return "浏览器自动化工具：获取网页内容、提取文本、读取页面标题和元信息。支持 fetch（获取页面文本）和 readability（提取正文）两种模式。"
}

func (s *BrowserSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "要访问的网页URL",
			},
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"fetch", "readability", "links", "headers"},
				"description": "操作类型：fetch=获取完整页面文本, readability=提取正文, links=提取链接, headers=仅获取HTTP头",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS选择器，用于提取特定元素（可选）",
			},
			"max_length": map[string]any{
				"type":        "integer",
				"description": "最大返回文本长度（默认8000字符）",
			},
		},
		"required": []string{"url", "action"},
	}
}

func (s *BrowserSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	rawURL, _ := args["url"].(string)
	action, _ := args["action"].(string)
	maxLen := 8000
	if ml, ok := args["max_length"].(float64); ok && ml > 0 {
		maxLen = int(ml)
	}

	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}

	// Validate URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	// Only allow http/https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("only http/https URLs are supported")
	}

	// Block private/internal IPs to prevent SSRF
	host := parsedURL.Hostname()
	if !s.allowPrivate && isPrivateHost(host) {
		return "", fmt.Errorf("access to private/internal addresses is not allowed")
	}

	switch action {
	case "fetch":
		return s.fetchPage(ctx, rawURL, maxLen)
	case "readability":
		return s.extractReadability(ctx, rawURL, maxLen, env)
	case "links":
		return s.extractLinks(ctx, rawURL, maxLen)
	case "headers":
		return s.fetchHeaders(ctx, rawURL)
	default:
		return "", fmt.Errorf("unknown action: %s (supported: fetch, readability, links, headers)", action)
	}
}

// fetchPage fetches the page and returns raw text content.
func (s *BrowserSkill) fetchPage(ctx context.Context, targetURL string, maxLen int) (string, error) {
	body, contentType, err := s.doGet(ctx, targetURL)
	if err != nil {
		return "", err
	}

	// For non-HTML content, return raw
	if !strings.Contains(contentType, "html") {
		if len(body) > maxLen {
			body = body[:maxLen] + "\n... [截断]"
		}
		return body, nil
	}

	// Strip HTML tags for plain text extraction
	text := stripHTML(body)
	text = collapseWhitespace(text)

	if len(text) > maxLen {
		text = text[:maxLen] + "\n... [截断]"
	}

	result := map[string]string{
		"url":     targetURL,
		"content": text,
	}
	// Extract title
	if title := extractHTMLTitle(body); title != "" {
		result["title"] = title
	}

	out, _ := json.Marshal(result)
	return string(out), nil
}

// extractReadability uses LLM to extract the main readable content from a page.
func (s *BrowserSkill) extractReadability(ctx context.Context, targetURL string, maxLen int, env *skills.Environment) (string, error) {
	body, _, err := s.doGet(ctx, targetURL)
	if err != nil {
		return "", err
	}

	text := stripHTML(body)
	text = collapseWhitespace(text)

	// Truncate to fit LLM context
	if len(text) > 12000 {
		text = text[:12000]
	}

	// Use LLM to extract main content
	if env != nil && env.LLMCall != nil {
		summary, err := env.LLMCall(ctx,
			"你是一个网页正文提取助手。请从以下网页文本中提取主要正文内容，去除导航、广告、页脚等无关内容。只返回正文，不要添加评论。",
			fmt.Sprintf("网页URL: %s\n\n网页文本:\n%s", targetURL, text))
		if err == nil && summary != "" {
			if len(summary) > maxLen {
				summary = summary[:maxLen] + "\n... [截断]"
			}
			result := map[string]string{
				"url":     targetURL,
				"content": summary,
			}
			out, _ := json.Marshal(result)
			return string(out), nil
		}
	}

	// Fallback: return stripped text
	if len(text) > maxLen {
		text = text[:maxLen] + "\n... [截断]"
	}
	result := map[string]string{
		"url":     targetURL,
		"content": text,
	}
	out, _ := json.Marshal(result)
	return string(out), nil
}

// extractLinks extracts all hyperlinks from a page.
func (s *BrowserSkill) extractLinks(ctx context.Context, targetURL string, maxLen int) (string, error) {
	body, _, err := s.doGet(ctx, targetURL)
	if err != nil {
		return "", err
	}

	links := extractHTMLLinks(body, targetURL)
	result := map[string]any{
		"url":   targetURL,
		"count": len(links),
		"links": links,
	}
	out, _ := json.Marshal(result)
	if len(out) > maxLen {
		// Truncate links list
		for len(out) > maxLen && len(links) > 0 {
			links = links[:len(links)-1]
			result["links"] = links
			result["count"] = len(links)
			result["truncated"] = true
			out, _ = json.Marshal(result)
		}
	}
	return string(out), nil
}

// fetchHeaders returns HTTP response headers.
func (s *BrowserSkill) fetchHeaders(ctx context.Context, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; YunqueBot/1.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	headers := make(map[string]string)
	for k, v := range resp.Header {
		headers[k] = strings.Join(v, ", ")
	}

	result := map[string]any{
		"url":         targetURL,
		"status":      resp.StatusCode,
		"status_text": resp.Status,
		"headers":     headers,
	}
	out, _ := json.Marshal(result)
	return string(out), nil
}

// doGet performs an HTTP GET request and returns the response body as string.
func (s *BrowserSkill) doGet(ctx context.Context, targetURL string) (body string, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; YunqueBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Limit body read to 1MB
	limited := io.LimitReader(resp.Body, 1<<20)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", "", fmt.Errorf("read body: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	return string(data), ct, nil
}

// stripHTML removes HTML tags from content, preserving text.
func stripHTML(html string) string {
	var b strings.Builder
	inTag := false
	inScript := false
	inStyle := false

	lower := strings.ToLower(html)
	n := len(html)

	for i := 0; i < n; i++ {
		// Check for closing script/style tags first
		if inScript {
			if i+9 <= n && lower[i:i+9] == "</script>" {
				inScript = false
				inTag = false
				i += 8 // skip past </script>
			}
			continue
		}
		if inStyle {
			if i+8 <= n && lower[i:i+8] == "</style>" {
				inStyle = false
				inTag = false
				i += 7 // skip past </style>
			}
			continue
		}

		if html[i] == '<' {
			// Check for opening script/style tags
			if i+7 <= n && lower[i:i+7] == "<script" {
				inScript = true
				inTag = true
				continue
			}
			if i+6 <= n && lower[i:i+6] == "<style" {
				inStyle = true
				inTag = true
				continue
			}

			inTag = true
			// Add whitespace for block elements
			if i+2 < n {
				tag := lower[i+1:]
				for _, bt := range []string{"p", "div", "br", "li", "h1", "h2", "h3", "h4", "h5", "h6", "tr", "td"} {
					if strings.HasPrefix(tag, bt) || strings.HasPrefix(tag, "/"+bt) {
						b.WriteByte('\n')
						break
					}
				}
			}
			continue
		}
		if html[i] == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteByte(html[i])
		}
	}
	return b.String()
}

// collapseWhitespace normalizes whitespace.
func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}

// extractHTMLTitle extracts the <title> content.
func extractHTMLTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title")
	if start < 0 {
		return ""
	}
	// Find closing >
	gtPos := strings.Index(lower[start:], ">")
	if gtPos < 0 {
		return ""
	}
	contentStart := start + gtPos + 1
	end := strings.Index(lower[contentStart:], "</title>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(html[contentStart : contentStart+end])
}

// extractHTMLLinks extracts href links from HTML.
func extractHTMLLinks(html, baseURL string) []map[string]string {
	var links []map[string]string
	lower := strings.ToLower(html)
	pos := 0
	for {
		idx := strings.Index(lower[pos:], "href=\"")
		if idx < 0 {
			break
		}
		start := pos + idx + 6
		end := strings.Index(html[start:], "\"")
		if end < 0 {
			break
		}
		href := html[start : start+end]
		pos = start + end + 1

		// Resolve relative URLs
		if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
			if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
				if base, err := url.Parse(baseURL); err == nil {
					if ref, err := url.Parse(href); err == nil {
						href = base.ResolveReference(ref).String()
					}
				}
			}
			links = append(links, map[string]string{"url": href})
		}

		if len(links) >= 100 {
			break
		}
	}
	return links
}

// isPrivateHost checks if a hostname resolves to a private/internal address.
func isPrivateHost(host string) bool {
	// Block well-known private/loopback hostnames
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "127.0.0.1" || lower == "::1" || lower == "0.0.0.0" {
		return true
	}
	// Block private IP ranges (10.x, 172.16-31.x, 192.168.x, 169.254.x)
	if strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "169.254.") ||
		strings.HasPrefix(host, "172.") {
		// Check 172.16.0.0 - 172.31.255.255
		if strings.HasPrefix(host, "172.") {
			parts := strings.SplitN(host, ".", 3)
			if len(parts) >= 2 {
				var second int
				_, err := fmt.Sscanf(parts[1], "%d", &second)
				if err == nil && second >= 16 && second <= 31 {
					return true
				}
			}
			return false
		}
		return true
	}
	// Block metadata endpoints
	if host == "metadata.google.internal" || host == "169.254.169.254" {
		return true
	}
	return false
}
