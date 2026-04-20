package gateway

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/integrations/mineru"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

func newTestGateway() (*Gateway, *tenant.Manager) {
	reg := skills.NewRegistry()
	llmClient := llm.NewClient("http://localhost:0", "test", "test")
	p := planner.NewPlanner(llmClient, reg, 8)
	tm := tenant.NewManager()
	short := memory.NewShortTerm(1 * time.Hour)
	mid := memory.NewMidTerm()
	long := memory.NewLongTerm()
	mm := memory.NewManager(short, mid, long)
	sched := scheduler.New(func(ctx context.Context, job scheduler.Job) {})
	cs := session.NewStore(50)
	pr := plugin.NewRegistry()
	jwtCfg := &JWTConfig{Secret: "test-secret", Issuer: "test", Expiration: time.Hour}
	return New(p, tm, mm, reg, sched, cs, pr, nil, nil, jwtCfg, nil, nil, nil), tm
}

func TestHealthz(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Fatalf("expected ok in body, got %s", w.Body.String())
	}
}

func TestCORSHeaders(t *testing.T) {
	gw, _ := newTestGateway()
	// CORS is opt-in: empty allowedOrigins means no CORS header is emitted
	// (safer default). Tests that want the legacy wildcard behaviour must
	// opt in explicitly.
	gw.SetAllowedOrigins([]string{"*"})
	req := httptest.NewRequest("OPTIONS", "/v1/chat", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}
}

func TestCORSDefaultsToSameOrigin(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("OPTIONS", "/v1/chat", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header without SetAllowedOrigins, got %q", got)
	}
}

func TestRequestID(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID header")
	}
}

func TestAuthRequired(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("GET", "/v1/skills", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}
}

func TestAuthWithAPIKey(t *testing.T) {
	gw, tm := newTestGateway()
	t1 := tm.Register("test-org")
	req := httptest.NewRequest("GET", "/v1/skills", nil)
	req.Header.Set("X-API-Key", t1.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 with valid key, got %d", w.Code)
	}
}

func TestAuthRejectsAPIKeyInQuery(t *testing.T) {
	gw, tm := newTestGateway()
	t1 := tm.Register("test-org")
	req := httptest.NewRequest("GET", "/v1/skills?api_key="+t1.APIKey, nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401 when api key is only present in query, got %d", w.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !rl.Allow("test") {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	if rl.Allow("test") {
		t.Fatal("4th request should be rate limited")
	}
}

func TestExtractKnowledgeHTML(t *testing.T) {
	raw := `<html><head><title>VS Code Codebase Overview</title><style>.x{}</style></head><body><main><h1>VS Code Codebase Overview</h1><p>This page describes the VS Code repository.</p><script>alert(1)</script></main></body></html>`
	text := extractKnowledgeHTML(raw)
	if !strings.Contains(text, "VS Code Codebase Overview") {
		t.Fatal("expected title text")
	}
	if !strings.Contains(text, "This page describes the VS Code repository.") {
		t.Fatal("expected body text")
	}
	if strings.Contains(text, "alert(1)") {
		t.Fatal("script content should be stripped")
	}
}

func TestExtractDeepWikiChildLinks(t *testing.T) {
	raw := `<a href="/microsoft/vscode/1-vs-code-codebase-overview">Overview</a><a href="https://deepwiki.com/microsoft/vscode/2-core-editor-(monaco)">Monaco</a><a href="/other/repo/page">Other</a><a href="#local">Skip</a>`
	links := extractDeepWikiChildLinks("https://deepwiki.com/microsoft/vscode", raw, 10)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0] != "https://deepwiki.com/microsoft/vscode/1-vs-code-codebase-overview" && links[1] != "https://deepwiki.com/microsoft/vscode/1-vs-code-codebase-overview" {
		t.Fatal("expected overview link")
	}
}

func TestBuildKnowledgeImportTree(t *testing.T) {
	root := &knowledgeImportPage{URL: "https://deepwiki.com/microsoft/vscode", Name: "VS Code"}
	imported := []*knowledge.Source{
		{Name: "VS Code", Path: "https://deepwiki.com/microsoft/vscode"},
		{Name: "Overview", Path: "https://deepwiki.com/microsoft/vscode/1-vs-code-codebase-overview"},
		{Name: "Startup", Path: "https://deepwiki.com/microsoft/vscode/1.1-application-startup-and-process-architecture"},
	}
	tree := buildKnowledgeImportTree(root, imported)
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 top-level chapter, got %d", len(tree.Children))
	}
	if tree.Children[0].Title != "Overview" {
		t.Fatalf("expected Overview title, got %s", tree.Children[0].Title)
	}
	if len(tree.Children[0].Children) != 1 || tree.Children[0].Children[0].Title != "Startup" {
		t.Fatal("expected nested Startup child")
	}
}

type stubDocumentParser struct {
	enabled bool
	result  *mineru.ParseResult
	err     error
}

func (s stubDocumentParser) Enabled() bool { return s.enabled }

func (s stubDocumentParser) ParseFile(ctx context.Context, filePath string) (*mineru.ParseResult, error) {
	return s.result, s.err
}

func TestIsMinerUSupportedExt(t *testing.T) {
	for _, ext := range []string{".pdf", ".docx", ".pptx", ".png", ".jpeg", ".tiff"} {
		if !isMinerUSupportedExt(ext) {
			t.Fatalf("expected supported ext: %s", ext)
		}
	}
	for _, ext := range []string{".txt", ".md", ".zip", ""} {
		if isMinerUSupportedExt(ext) {
			t.Fatalf("expected unsupported ext: %s", ext)
		}
	}
}

func TestIngestKnowledgeWithMinerU(t *testing.T) {
	g := &Gateway{
		knowledgeStore: knowledge.NewStore(200),
		documentParser: stubDocumentParser{
			enabled: true,
			result:  &mineru.ParseResult{Backend: "cli", Markdown: "# Parsed\n\nHello MinerU", JSON: `{}`},
		},
	}

	parsed, err := g.parseFileWithMinerU(context.Background(), "demo.pdf", []byte("pdf bytes"))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed.Parse["parser"] != "mineru" {
		t.Fatalf("expected parser metadata, got %#v", parsed.Parse)
	}
	if parsed.Parse["preview"] == "" {
		t.Fatalf("expected preview metadata, got %#v", parsed.Parse)
	}

	res, err := g.ingestKnowledgeWithMinerU(context.Background(), "demo.pdf", []byte("pdf bytes"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Source == nil {
		t.Fatal("expected knowledge source")
	}
	if res.Source.Name != "demo.md" {
		t.Fatalf("expected converted markdown source name, got %s", res.Source.Name)
	}
	if res.Source.Path != "demo.pdf" {
		t.Fatalf("expected original path recorded, got %s", res.Source.Path)
	}
	if res.Parse["parser"] != "mineru" {
		t.Fatalf("expected parser metadata, got %#v", res.Parse)
	}
	if got := res.Parse["has_layout_json"]; got != true {
		t.Fatalf("expected layout json metadata, got %#v", got)
	}
	if len(g.knowledgeStore.Sources()) != 1 {
		t.Fatalf("expected source ingested, got %d", len(g.knowledgeStore.Sources()))
	}
}
