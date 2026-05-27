package tori

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestValidateToriTargetRejectsPrivateAndMetadataHosts(t *testing.T) {
	cases := []string{
		"http://127.0.0.1:8080/oauth/token",
		"http://localhost:8080/oauth/token",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/internal",
		"http://172.16.0.5/internal",
		"http://192.168.1.10/internal",
	}

	for _, raw := range cases {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if err := validateToriTarget(u); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestSafeHTTPClientBlocksRedirectToLoopback(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1/internal", http.StatusFound)
	}))
	defer redirector.Close()

	client := newSafeHTTPClient(time.Second)
	req, err := http.NewRequest(http.MethodGet, redirector.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected redirect to loopback to be blocked")
	}
	if !strings.Contains(err.Error(), "private") && !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected private/blocked redirect error, got %v", err)
	}
}

func TestPostSafeFormRejectsLoopbackBeforeDial(t *testing.T) {
	_, err := postSafeForm(t.Context(), "http://127.0.0.1/oauth/token", url.Values{"grant_type": {"refresh_token"}}, time.Second)
	if err == nil {
		t.Fatal("expected loopback token endpoint to be rejected")
	}
}

func TestSyncClientRejectsUnsafeBaseURL(t *testing.T) {
	client := NewSyncClient("http://127.0.0.1:8080", "token", "passphrase")
	if err := client.Push(t.Context(), "memory", []byte("secret"), 1); err == nil {
		t.Fatal("expected unsafe sync base URL to be rejected")
	}
}
