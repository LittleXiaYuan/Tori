package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestChatRetryOnFailure(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server error"}`))
			return
		}
		resp := ChatResponse{Choices: []struct {
			Message Message `json:"message"`
		}{{Message: Message{Role: "assistant", Content: "ok"}}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "test-model")
	reply, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if reply != "ok" {
		t.Fatalf("expected 'ok', got %q", reply)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestChatAllRetriesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"always fail"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "test-model")
	_, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7)
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
}

func TestChatSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{Choices: []struct {
			Message Message `json:"message"`
		}{{Message: Message{Role: "assistant", Content: "hello"}}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "test-model")
	reply, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "hello" {
		t.Fatalf("expected 'hello', got %q", reply)
	}
}
