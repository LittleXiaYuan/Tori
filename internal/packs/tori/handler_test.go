package toripack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/tori"
	"yunque-agent/pkg/packruntime"
)

type fakeStore struct {
	token    *tori.StoredToken
	clearErr error
	storeErr error
}

func (f *fakeStore) IsBound() bool { return f.token != nil }

func (f *fakeStore) Get() *tori.StoredToken {
	if f.token == nil {
		return nil
	}
	cp := *f.token
	return &cp
}

func (f *fakeStore) Store(tok *tori.TokenResponse, user *tori.UserInfo, base string) error {
	if f.storeErr != nil {
		return f.storeErr
	}
	f.token = &tori.StoredToken{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
		ToriBaseURL:  base,
	}
	if user != nil {
		f.token.UserID = user.UserID
		f.token.Username = user.Username
		f.token.Email = user.Email
		f.token.APIKey = user.APIKey
	}
	return nil
}

func (f *fakeStore) Clear() error {
	if f.clearErr != nil {
		return f.clearErr
	}
	f.token = nil
	return nil
}

func TestToriPackV2AndRouteSpecs(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := NewProvider(nil)
	if h.PackID() != PackID {
		t.Fatalf("PackID=%q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 5 {
		t.Fatalf("Routes len=%d, want 5", got)
	}
	if got := len(RouteSpecs()); got != 5 {
		t.Fatalf("RouteSpecs len=%d, want 5", got)
	}
	routes := map[string]map[string]bool{}
	for _, route := range h.Routes() {
		if routes[route.Path] == nil {
			routes[route.Path] = map[string]bool{}
		}
		routes[route.Path][route.Method] = true
	}
	for _, spec := range RouteSpecs() {
		if !routes[spec.Path][spec.Method] {
			t.Fatalf("routeSpec %s %s not mounted", spec.Method, spec.Path)
		}
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestValidateToriURLRejectsMetadataTarget(t *testing.T) {
	if _, err := ValidateToriURL("http://169.254.169.254/latest/meta-data/"); err == nil {
		t.Fatal("expected metadata Tori URL to be rejected")
	}
}

func TestToriStatusNilStore(t *testing.T) {
	h := NewProvider(nil)
	rec := httptest.NewRecorder()
	h.Status(rec, httptest.NewRequest(http.MethodGet, "/v1/tori/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body tori.BindingStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Bound {
		t.Fatalf("expected unbound status, got %+v", body)
	}
}

func TestToriBindStartsFlowWithoutOpeningRealBrowser(t *testing.T) {
	store := &fakeStore{}
	started := false
	opened := ""
	h := NewProvider(func() tokenStore { return store }).
		WithBindStarter(func(ctx context.Context, cfg tori.OAuthConfig) (string, <-chan tori.BindResult, error) {
			started = true
			ch := make(chan tori.BindResult)
			close(ch)
			return "https://tori.example/oauth", ch, nil
		}).
		WithBrowserOpener(func(url string) { opened = url })

	rec := httptest.NewRecorder()
	h.Bind(rec, httptest.NewRequest(http.MethodPost, "/v1/tori/bind", strings.NewReader(`{"tori_url":"https://example.com"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !started || opened != "https://tori.example/oauth" {
		t.Fatalf("bind flow not invoked correctly, started=%v opened=%q", started, opened)
	}
}

func TestToriHealthAndUsageUseInjectedClients(t *testing.T) {
	store := &fakeStore{
		token: &tori.StoredToken{
			AccessToken:  "tok",
			RefreshToken: "ref",
			ExpiresAt:    time.Now().Add(time.Hour),
			APIKey:       "key",
			ToriBaseURL:  "https://tori.example",
		},
	}
	h := NewProvider(func() tokenStore { return store }).
		WithHealthChecker(func(base string) (*tori.HealthStatus, error) {
			if base != "https://tori.example" {
				t.Fatalf("health base=%q", base)
			}
			return &tori.HealthStatus{Status: "ok", Version: "test"}, nil
		}).
		WithUsageFetcher(func(base, apiKey string) (*tori.UsageSummary, error) {
			if base != "https://tori.example" || apiKey != "key" {
				t.Fatalf("usage base=%q key=%q", base, apiKey)
			}
			return &tori.UsageSummary{RequestCount: 7}, nil
		})

	health := httptest.NewRecorder()
	h.Health(health, httptest.NewRequest(http.MethodGet, "/v1/tori/health", nil))
	if !strings.Contains(health.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health body: %s", health.Body.String())
	}

	usage := httptest.NewRecorder()
	h.Usage(usage, httptest.NewRequest(http.MethodGet, "/v1/tori/usage", nil))
	if !strings.Contains(usage.Body.String(), `"request_count":7`) {
		t.Fatalf("unexpected usage body: %s", usage.Body.String())
	}
}
