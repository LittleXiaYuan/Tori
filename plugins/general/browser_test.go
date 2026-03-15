package general

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestBrowserSkillName(t *testing.T) {
	s := NewBrowserSkill()
	if s.Name() != "browser" {
		t.Fatalf("expected browser, got %s", s.Name())
	}
}

// newTestBrowserSkill creates a BrowserSkill with private address access allowed for httptest servers.
func newTestBrowserSkill() *BrowserSkill {
	s := NewBrowserSkill()
	s.allowPrivate = true
	return s
}

func TestBrowserSkillParameters(t *testing.T) {
	s := NewBrowserSkill()
	params := s.Parameters()
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing properties")
	}
	for _, key := range []string{"url", "action"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("missing parameter: %s", key)
		}
	}
	required, _ := params["required"].([]string)
	if len(required) != 2 {
		t.Fatalf("expected 2 required params, got %d", len(required))
	}
}

func TestBrowserSkillFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Test Page</title></head><body><h1>Hello</h1><p>World</p></body></html>`)
	}))
	defer srv.Close()

	s := newTestBrowserSkill()
	result, err := s.Execute(context.Background(), map[string]any{
		"url":    srv.URL,
		"action": "fetch",
	}, nil)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["title"] != "Test Page" {
		t.Fatalf("expected title 'Test Page', got %q", parsed["title"])
	}
	if !strings.Contains(parsed["content"], "Hello") {
		t.Fatalf("expected content to contain Hello: %q", parsed["content"])
	}
	if !strings.Contains(parsed["content"], "World") {
		t.Fatalf("expected content to contain World: %q", parsed["content"])
	}
}

func TestBrowserSkillFetchPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "plain text content")
	}))
	defer srv.Close()

	s := newTestBrowserSkill()
	result, err := s.Execute(context.Background(), map[string]any{
		"url":    srv.URL,
		"action": "fetch",
	}, nil)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if result != "plain text content" {
		t.Fatalf("expected plain text content, got %q", result)
	}
}

func TestBrowserSkillHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := newTestBrowserSkill()
	result, err := s.Execute(context.Background(), map[string]any{
		"url":    srv.URL,
		"action": "headers",
	}, nil)
	if err != nil {
		t.Fatalf("headers failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	if parsed["status"].(float64) != 200 {
		t.Fatalf("expected status 200, got %v", parsed["status"])
	}
	headers := parsed["headers"].(map[string]any)
	if headers["X-Custom-Header"] != "test-value" {
		t.Fatalf("expected custom header, got %v", headers["X-Custom-Header"])
	}
}

func TestBrowserSkillLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<a href="https://example.com">Example</a>
			<a href="/page2">Page 2</a>
			<a href="#anchor">Anchor</a>
			<a href="javascript:void(0)">JS</a>
		</body></html>`)
	}))
	defer srv.Close()

	s := newTestBrowserSkill()
	result, err := s.Execute(context.Background(), map[string]any{
		"url":    srv.URL,
		"action": "links",
	}, nil)
	if err != nil {
		t.Fatalf("links failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	links := parsed["links"].([]any)
	// Should have 2 links (example.com and /page2 resolved), anchor and javascript excluded
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d: %v", len(links), links)
	}
}

func TestBrowserSkillReadability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<nav>Menu Item 1 | Menu Item 2</nav>
			<article>
				<h1>Article Title</h1>
				<p>This is the main article content that should be extracted.</p>
			</article>
			<footer>Copyright 2024</footer>
		</body></html>`)
	}))
	defer srv.Close()

	s := newTestBrowserSkill()

	// Without LLM — should fallback to stripped text
	result, err := s.Execute(context.Background(), map[string]any{
		"url":    srv.URL,
		"action": "readability",
	}, nil)
	if err != nil {
		t.Fatalf("readability failed: %v", err)
	}

	var parsed map[string]string
	json.Unmarshal([]byte(result), &parsed)
	if !strings.Contains(parsed["content"], "Article Title") {
		t.Fatalf("expected content to contain Article Title: %q", parsed["content"])
	}

	// With mock LLM
	mockEnv := &skills.Environment{
		LLMCall: func(ctx context.Context, system, user string) (string, error) {
			return "LLM提取的正文内容", nil
		},
	}
	result2, err := s.Execute(context.Background(), map[string]any{
		"url":    srv.URL,
		"action": "readability",
	}, mockEnv)
	if err != nil {
		t.Fatalf("readability with LLM failed: %v", err)
	}

	var parsed2 map[string]string
	json.Unmarshal([]byte(result2), &parsed2)
	if parsed2["content"] != "LLM提取的正文内容" {
		t.Fatalf("expected LLM content, got %q", parsed2["content"])
	}
}

func TestBrowserSkillSSRFProtection(t *testing.T) {
	s := NewBrowserSkill()

	badURLs := []string{
		"http://localhost/secret",
		"http://127.0.0.1/admin",
		"http://10.0.0.1/internal",
		"http://192.168.1.1/router",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/local",
		"ftp://example.com/file",
		"file:///etc/passwd",
	}

	for _, u := range badURLs {
		_, err := s.Execute(context.Background(), map[string]any{
			"url":    u,
			"action": "fetch",
		}, nil)
		if err == nil {
			t.Fatalf("expected error for URL %s", u)
		}
	}
}

func TestBrowserSkillMaxLength(t *testing.T) {
	longContent := strings.Repeat("X", 10000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, longContent)
	}))
	defer srv.Close()

	s := newTestBrowserSkill()
	result, err := s.Execute(context.Background(), map[string]any{
		"url":        srv.URL,
		"action":     "fetch",
		"max_length": float64(100),
	}, nil)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(result) > 200 { // 100 + truncation notice
		t.Fatalf("expected truncated result, got length %d", len(result))
	}
	if !strings.Contains(result, "[截断]") {
		t.Fatalf("expected truncation notice")
	}
}

func TestBrowserSkillInvalidAction(t *testing.T) {
	s := NewBrowserSkill()
	_, err := s.Execute(context.Background(), map[string]any{
		"url":    "https://example.com",
		"action": "invalid",
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestBrowserSkillMissingURL(t *testing.T) {
	s := NewBrowserSkill()
	_, err := s.Execute(context.Background(), map[string]any{
		"url":    "",
		"action": "fetch",
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<script>alert(1)</script>World", "World"},
		{"<style>.x{color:red}</style>Text", "Text"},
		{"<div>A</div><div>B</div>", "A\nB"},
		{"No tags here", "No tags here"},
	}
	for _, tt := range tests {
		got := collapseWhitespace(stripHTML(tt.input))
		if !strings.Contains(got, tt.expect) {
			t.Errorf("stripHTML(%q): got %q, want to contain %q", tt.input, got, tt.expect)
		}
	}
}

func TestExtractHTMLTitle(t *testing.T) {
	html := `<html><head><title>My Page Title</title></head><body>content</body></html>`
	title := extractHTMLTitle(html)
	if title != "My Page Title" {
		t.Fatalf("expected 'My Page Title', got %q", title)
	}

	// No title
	title2 := extractHTMLTitle("<html><body>no title</body></html>")
	if title2 != "" {
		t.Fatalf("expected empty title, got %q", title2)
	}
}

func TestIsPrivateHost(t *testing.T) {
	privates := []string{"localhost", "127.0.0.1", "::1", "0.0.0.0", "10.0.0.1", "192.168.1.1", "172.16.0.1", "172.31.255.255", "169.254.169.254", "metadata.google.internal"}
	for _, h := range privates {
		if !isPrivateHost(h) {
			t.Errorf("expected %s to be private", h)
		}
	}

	publics := []string{"example.com", "8.8.8.8", "172.32.0.1", "172.15.0.1"}
	for _, h := range publics {
		if isPrivateHost(h) {
			t.Errorf("expected %s to be public", h)
		}
	}
}
