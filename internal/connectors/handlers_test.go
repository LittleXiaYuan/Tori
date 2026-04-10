package connectors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type rewriteTransport struct {
	base *url.URL
	rt   http.RoundTripper
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.base.Scheme
	clone.URL.Host = t.base.Host
	if t.rt == nil {
		t.rt = http.DefaultTransport
	}
	return t.rt.RoundTrip(clone)
}

func TestGenericRESTHandlerValidateRejectsAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "bad token",
		})
	}))
	defer srv.Close()

	h := NewGenericRESTHandler(srv.URL, "Authorization", "Bearer ", srv.URL, "user")
	h.SuccessField = "ok"

	ok, _, err := h.Validate(context.Background(), &Credentials{AccessToken: "token"})
	if err == nil || !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("expected upstream validation error, got ok=%v err=%v", ok, err)
	}
	if ok {
		t.Fatalf("expected validation failure")
	}
}

func TestGenericRESTHandlerExecuteRejectsAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "channel_not_found",
		})
	}))
	defer srv.Close()

	h := NewSlackHandler()
	h.BaseURL = srv.URL

	_, err := h.Execute(context.Background(), &Credentials{AccessToken: "token"}, "send_message", map[string]any{
		"channel": "C123",
		"text":    "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "channel_not_found") {
		t.Fatalf("expected execute error, got %v", err)
	}
}

func TestNotionHandlerValidateSetsRequiredHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Notion-Version"); got != notionVersion {
			t.Fatalf("unexpected Notion-Version header: %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		if r.URL.Path != "/v1/users/me" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "Workspace Bot"})
	}))
	defer srv.Close()

	base, _ := url.Parse(srv.URL)
	h := NewNotionHandler()
	h.httpClient = &http.Client{Transport: rewriteTransport{base: base}}

	ok, user, err := h.Validate(context.Background(), &Credentials{AccessToken: "secret"})
	if err != nil || !ok {
		t.Fatalf("validate failed: ok=%v err=%v", ok, err)
	}
	if user != "Workspace Bot" {
		t.Fatalf("unexpected user info: %q", user)
	}
}

func TestLinearHandlerUsesGraphQLEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/graphql" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "lin_api_key" {
			t.Fatalf("unexpected auth header: %q", got)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "viewer") {
			t.Fatalf("expected GraphQL viewer query, got %s", string(body))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"viewer": map[string]any{
					"name": "Linear Bot",
				},
			},
		})
	}))
	defer srv.Close()

	base, _ := url.Parse(srv.URL)
	h := NewLinearHandler()
	h.httpClient = &http.Client{Transport: rewriteTransport{base: base}}

	ok, user, err := h.Validate(context.Background(), &Credentials{AccessToken: "lin_api_key"})
	if err != nil || !ok {
		t.Fatalf("validate failed: ok=%v err=%v", ok, err)
	}
	if user != "Linear Bot" {
		t.Fatalf("unexpected user info: %q", user)
	}
}
