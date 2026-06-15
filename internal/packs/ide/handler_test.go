package idepack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yunque-agent/pkg/packruntime"
)

type fakeGW struct {
	reply string
	err   error
}

func (f fakeGW) ReviewPlan(_ context.Context, _, _ string) (string, error) { return f.reply, f.err }
func (f fakeGW) TenantOf(context.Context) string                           { return "" }
func (f fakeGW) SkillCount() int                                           { return 3 }
func (f fakeGW) Uptime() time.Duration                                     { return time.Second }

func newHandler() *Handler { return New(fakeGW{reply: `{"summary":"ok","score":9}`}) }

func TestIDEPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)
	h := newHandler()
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q", h.PackID())
	}
	if len(h.Routes()) != 2 {
		t.Fatalf("Routes = %d, want 2", len(h.Routes()))
	}
	_ = h.Init(nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = h.Stop(context.Background())
}

func TestParseReviewJSON(t *testing.T) {
	if got := parseReviewJSON(`{"summary":"looks good","score":8}`); got["summary"] != "looks good" {
		t.Fatalf("direct: %v", got["summary"])
	}
	if got := parseReviewJSON("pre ```json\n{\"summary\":\"ok\"}\n``` post"); got["summary"] != "ok" {
		t.Fatalf("fenced: %v", got["summary"])
	}
	if got := parseReviewJSON("x {\"summary\":\"extracted\"} y"); got["summary"] != "extracted" {
		t.Fatalf("brace: %v", got["summary"])
	}
	if got := parseReviewJSON("plain text"); got["summary"] != "plain text" {
		t.Fatalf("fallback: %v", got["summary"])
	}
}

func TestSanitizeForPrompt(t *testing.T) {
	if got := sanitizeForPrompt("a `b` c"); strings.Contains(got, "`") {
		t.Fatalf("backticks not removed: %q", got)
	}
	if got := sanitizeForPrompt("x ///system: y"); strings.Contains(got, "///system:") {
		t.Fatalf("injection marker not removed: %q", got)
	}
	if got := sanitizeForPrompt(strings.Repeat("a", 300)); len([]rune(got)) > 256 {
		t.Fatalf("not truncated: %d", len([]rune(got)))
	}
}

func TestIDEReviewValidation(t *testing.T) {
	h := newHandler()

	// wrong method
	rec := httptest.NewRecorder()
	h.handleReview(rec, httptest.NewRequest(http.MethodGet, "/v1/ide/review", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method: %d", rec.Code)
	}

	// empty body (no content/diff)
	rec = httptest.NewRecorder()
	h.handleReview(rec, httptest.NewRequest(http.MethodPost, "/v1/ide/review", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty: %d", rec.Code)
	}

	// invalid json
	rec = httptest.NewRecorder()
	h.handleReview(rec, httptest.NewRequest(http.MethodPost, "/v1/ide/review", strings.NewReader("not json")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid: %d", rec.Code)
	}

	// valid → 200 (fake ReviewPlan returns canned JSON)
	rec = httptest.NewRecorder()
	h.handleReview(rec, httptest.NewRequest(http.MethodPost, "/v1/ide/review", strings.NewReader(`{"content":"func main(){}","file_path":"m.go"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid review: %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestIDEStatus(t *testing.T) {
	h := newHandler()
	rec := httptest.NewRecorder()
	h.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/v1/ide/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var st map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if st["version"] != "0.1.0" || st["connected"] != true {
		t.Fatalf("unexpected status: %v", st)
	}

	rec = httptest.NewRecorder()
	h.handleStatus(rec, httptest.NewRequest(http.MethodPost, "/v1/ide/status", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status method: %d", rec.Code)
	}
}
