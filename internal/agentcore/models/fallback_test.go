package models

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

// ──────────────────────────────────────────────
// Mock provider
// ──────────────────────────────────────────────

type mockProvider struct {
	id      string
	handler func(ctx context.Context, profile *AuthProfile, req *CompletionRequest) (*CompletionResponse, error)
}

func (m *mockProvider) ID() string { return m.id }
func (m *mockProvider) Complete(ctx context.Context, profile *AuthProfile, req *CompletionRequest) (*CompletionResponse, error) {
	return m.handler(ctx, profile, req)
}

// ──────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────

func TestCompleteSuccess(t *testing.T) {
	fc := NewFallbackChain()
	fc.RegisterProvider(&mockProvider{
		id: "openai",
		handler: func(ctx context.Context, p *AuthProfile, req *CompletionRequest) (*CompletionResponse, error) {
			return &CompletionResponse{Model: req.Model, Content: "hello"}, nil
		},
	})
	fc.AddProfile(AuthProfile{ID: "p1", Provider: "openai", APIKey: "sk-test", Enabled: true})
	fc.AddModel(ModelEntry{ID: "gpt-4", Provider: "openai", ProfileID: "p1"})

	resp, err := fc.Complete(context.Background(), &CompletionRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Fatalf("expected hello, got %s", resp.Content)
	}
	if resp.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", resp.Attempts)
	}
}

func TestFallbackToSecondModel(t *testing.T) {
	var calls int32
	fc := NewFallbackChain()
	fc.RegisterProvider(&mockProvider{
		id: "openai",
		handler: func(ctx context.Context, p *AuthProfile, req *CompletionRequest) (*CompletionResponse, error) {
			n := atomic.AddInt32(&calls, 1)
			if req.Model == "gpt-4" {
				return nil, fmt.Errorf("rate limited")
			}
			return &CompletionResponse{Model: req.Model, Content: fmt.Sprintf("ok from attempt %d", n)}, nil
		},
	})
	fc.AddProfile(AuthProfile{ID: "p1", Provider: "openai", APIKey: "sk-test", Enabled: true})
	fc.AddModel(ModelEntry{ID: "gpt-4", Provider: "openai", ProfileID: "p1", Fallbacks: []string{"gpt-3.5"}, MaxRetries: 1})
	fc.AddModel(ModelEntry{ID: "gpt-3.5", Provider: "openai", ProfileID: "p1", MaxRetries: 1})

	resp, err := fc.Complete(context.Background(), &CompletionRequest{Model: "gpt-4"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "gpt-3.5" {
		t.Fatalf("expected gpt-3.5, got %s", resp.Model)
	}
}

func TestProfileRotation(t *testing.T) {
	var usedProfiles []string
	fc := NewFallbackChain()
	fc.RegisterProvider(&mockProvider{
		id: "openai",
		handler: func(ctx context.Context, p *AuthProfile, req *CompletionRequest) (*CompletionResponse, error) {
			if p != nil {
				usedProfiles = append(usedProfiles, p.ID)
			}
			if p != nil && p.ID == "p1" {
				return nil, fmt.Errorf("p1 expired")
			}
			return &CompletionResponse{Model: req.Model, Content: "ok"}, nil
		},
	})
	fc.AddProfile(AuthProfile{ID: "p1", Name: "primary", Provider: "openai", APIKey: "sk-1", Enabled: true})
	fc.AddProfile(AuthProfile{ID: "p2", Name: "backup", Provider: "openai", APIKey: "sk-2", Enabled: true})
	fc.AddModel(ModelEntry{ID: "gpt-4", Provider: "openai", ProfileID: "p1", MaxRetries: 1})

	resp, err := fc.Complete(context.Background(), &CompletionRequest{Model: "gpt-4"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ProfileID != "p2" {
		t.Fatalf("expected p2 profile, got %s", resp.ProfileID)
	}
}

func TestAllExhausted(t *testing.T) {
	fc := NewFallbackChain()
	fc.RegisterProvider(&mockProvider{
		id: "openai",
		handler: func(ctx context.Context, p *AuthProfile, req *CompletionRequest) (*CompletionResponse, error) {
			return nil, fmt.Errorf("always fails")
		},
	})
	fc.AddProfile(AuthProfile{ID: "p1", Provider: "openai", APIKey: "sk-test", Enabled: true})
	fc.AddModel(ModelEntry{ID: "gpt-4", Provider: "openai", ProfileID: "p1", MaxRetries: 2})

	_, err := fc.Complete(context.Background(), &CompletionRequest{Model: "gpt-4"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNoProvider(t *testing.T) {
	fc := NewFallbackChain()
	fc.AddModel(ModelEntry{ID: "gpt-4", Provider: "missing"})

	_, err := fc.Complete(context.Background(), &CompletionRequest{Model: "gpt-4"})
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestListModels(t *testing.T) {
	fc := NewFallbackChain()
	fc.AddModel(ModelEntry{ID: "a"})
	fc.AddModel(ModelEntry{ID: "b"})
	if len(fc.ListModels()) != 2 {
		t.Fatal("expected 2 models")
	}
}

func TestListProfilesMasked(t *testing.T) {
	fc := NewFallbackChain()
	fc.AddProfile(AuthProfile{ID: "p1", APIKey: "sk-1234567890abcdef"})
	profiles := fc.ListProfiles()
	if len(profiles) != 1 {
		t.Fatal("expected 1 profile")
	}
	if profiles[0].APIKey == "sk-1234567890abcdef" {
		t.Fatal("api key should be masked")
	}
	if profiles[0].APIKey != "sk-1...cdef" {
		t.Fatalf("unexpected mask: %s", profiles[0].APIKey)
	}
}

func TestDisabledProfile(t *testing.T) {
	fc := NewFallbackChain()
	fc.RegisterProvider(&mockProvider{
		id: "openai",
		handler: func(ctx context.Context, p *AuthProfile, req *CompletionRequest) (*CompletionResponse, error) {
			return &CompletionResponse{Model: req.Model, Content: "ok", ProfileID: p.ID}, nil
		},
	})
	fc.AddProfile(AuthProfile{ID: "disabled", Provider: "openai", APIKey: "sk-x", Enabled: false})
	fc.AddProfile(AuthProfile{ID: "enabled", Provider: "openai", APIKey: "sk-y", Enabled: true})
	fc.AddModel(ModelEntry{ID: "gpt-4", Provider: "openai", ProfileID: "disabled"})

	resp, err := fc.Complete(context.Background(), &CompletionRequest{Model: "gpt-4"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ProfileID != "enabled" {
		t.Fatalf("should use enabled profile, got %s", resp.ProfileID)
	}
}

func TestDefaultRetries(t *testing.T) {
	fc := NewFallbackChain()
	fc.AddModel(ModelEntry{ID: "test"})
	m := fc.models["test"]
	if m.MaxRetries != 2 {
		t.Fatalf("expected default 2 retries, got %d", m.MaxRetries)
	}
}
