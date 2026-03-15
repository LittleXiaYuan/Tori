package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── Mock Ollama /api/tags server ──

func mockOllamaServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "llama3:8b", "model": "llama3:8b", "size": 4700000000, "modified_at": "2024-01-01T00:00:00Z"},
				{"name": "qwen2:7b", "model": "qwen2:7b", "size": 3900000000, "modified_at": "2024-01-02T00:00:00Z"},
			},
		})
	})
	return httptest.NewServer(mux)
}

// ── Mock vLLM /v1/models server ──

func mockVLLMServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "meta-llama/Llama-3-8B", "object": "model", "owned_by": "vllm"},
			},
		})
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version": "vllm 0.4.0"}`))
	})
	return httptest.NewServer(mux)
}

func TestDiscoverOllama(t *testing.T) {
	srv := mockOllamaServer()
	defer srv.Close()

	models, err := discoverOllama(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("discoverOllama: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "llama3:8b" {
		t.Errorf("model[0].ID = %q, want llama3:8b", models[0].ID)
	}
	if models[0].Backend != BackendOllama {
		t.Errorf("model[0].Backend = %q, want ollama", models[0].Backend)
	}
	if models[1].ID != "qwen2:7b" {
		t.Errorf("model[1].ID = %q, want qwen2:7b", models[1].ID)
	}
}

func TestDiscoverOpenAICompat(t *testing.T) {
	srv := mockVLLMServer()
	defer srv.Close()

	models, err := discoverOpenAICompat(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("discoverOpenAICompat: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].ID != "meta-llama/Llama-3-8B" {
		t.Errorf("model[0].ID = %q, want meta-llama/Llama-3-8B", models[0].ID)
	}
}

func TestProbeLocalOllama(t *testing.T) {
	srv := mockOllamaServer()
	defer srv.Close()

	result := ProbeLocal(context.Background(), srv.URL)
	if !result.Available {
		t.Fatalf("expected available, got error: %s", result.Error)
	}
	if result.Backend != BackendOllama {
		t.Errorf("backend = %q, want ollama", result.Backend)
	}
	if len(result.Models) != 2 {
		t.Errorf("models count = %d, want 2", len(result.Models))
	}
	if result.Latency < 0 {
		t.Error("latency should be non-negative")
	}
}

func TestProbeLocalVLLM(t *testing.T) {
	srv := mockVLLMServer()
	defer srv.Close()

	result := ProbeLocal(context.Background(), srv.URL)
	if !result.Available {
		t.Fatalf("expected available, got error: %s", result.Error)
	}
	if result.Backend != BackendVLLM {
		t.Errorf("backend = %q, want vllm", result.Backend)
	}
	if len(result.Models) != 1 {
		t.Errorf("models count = %d, want 1", len(result.Models))
	}
}

func TestProbeLocalUnreachable(t *testing.T) {
	result := ProbeLocal(context.Background(), "http://127.0.0.1:1")
	if result.Available {
		t.Error("expected unavailable for unreachable host")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestAutoRegisterLocalOllama(t *testing.T) {
	srv := mockOllamaServer()
	defer srv.Close()

	pool := NewPool()
	pool.Register("primary", NewClient("http://example.com", "key", "model"))
	reg := NewProviderRegistry(pool)

	pid, err := AutoRegisterLocal(context.Background(), reg, LocalAutoConfig{
		BaseURL: srv.URL,
		Tier:    "fast",
		Backend: BackendOllama,
	})
	if err != nil {
		t.Fatalf("AutoRegisterLocal: %v", err)
	}
	if pid != "local-ollama" {
		t.Errorf("provider id = %q, want local-ollama", pid)
	}

	// Check provider is in the registry
	providers := reg.List()
	found := false
	for _, p := range providers {
		if p.ID == "local-ollama" {
			found = true
			if p.Model != "llama3:8b" {
				t.Errorf("model = %q, want llama3:8b (first discovered)", p.Model)
			}
			if p.Tier != "fast" {
				t.Errorf("tier = %q, want fast", p.Tier)
			}
		}
	}
	if !found {
		t.Error("local-ollama provider not found in registry")
	}
}

func TestAutoRegisterLocalSpecificModel(t *testing.T) {
	srv := mockOllamaServer()
	defer srv.Close()

	pool := NewPool()
	pool.Register("primary", NewClient("http://example.com", "key", "model"))
	reg := NewProviderRegistry(pool)

	pid, err := AutoRegisterLocal(context.Background(), reg, LocalAutoConfig{
		BaseURL: srv.URL,
		Model:   "qwen2:7b",
		Tier:    "smart",
		Backend: BackendOllama,
	})
	if err != nil {
		t.Fatalf("AutoRegisterLocal: %v", err)
	}
	if pid != "local-ollama" {
		t.Errorf("provider id = %q", pid)
	}
	p := reg.Get("local-ollama")
	if p == nil {
		t.Fatal("provider not found")
	}
	if p.Config.Model != "qwen2:7b" {
		t.Errorf("model = %q, want qwen2:7b", p.Config.Model)
	}
}

func TestAutoRegisterLocalVLLM(t *testing.T) {
	srv := mockVLLMServer()
	defer srv.Close()

	pool := NewPool()
	pool.Register("primary", NewClient("http://example.com", "key", "model"))
	reg := NewProviderRegistry(pool)

	pid, err := AutoRegisterLocal(context.Background(), reg, LocalAutoConfig{
		BaseURL: srv.URL,
		Tier:    "fast",
		Backend: BackendVLLM,
	})
	if err != nil {
		t.Fatalf("AutoRegisterLocal: %v", err)
	}
	if pid != "local-vllm" {
		t.Errorf("provider id = %q, want local-vllm", pid)
	}
}

func TestAutoRegisterLocalFail(t *testing.T) {
	pool := NewPool()
	pool.Register("primary", NewClient("http://example.com", "key", "model"))
	reg := NewProviderRegistry(pool)

	_, err := AutoRegisterLocal(context.Background(), reg, LocalAutoConfig{
		BaseURL: "http://127.0.0.1:1",
		Tier:    "fast",
	})
	if err == nil {
		t.Error("expected error for unreachable backend")
	}
}

func TestChatBaseURL(t *testing.T) {
	tests := []struct {
		base    string
		backend LocalBackend
		want    string
	}{
		{"http://localhost:11434", BackendOllama, "http://localhost:11434/v1"},
		{"http://localhost:8000", BackendVLLM, "http://localhost:8000/v1"},
		{"http://localhost:8000/v1", BackendVLLM, "http://localhost:8000/v1"},
		{"http://localhost:1234", BackendLMStudio, "http://localhost:1234/v1"},
	}
	for _, tt := range tests {
		got := chatBaseURL(tt.base, tt.backend)
		if got != tt.want {
			t.Errorf("chatBaseURL(%q, %q) = %q, want %q", tt.base, tt.backend, got, tt.want)
		}
	}
}

func TestDiscoverModels(t *testing.T) {
	srv := mockOllamaServer()
	defer srv.Close()

	models, err := DiscoverModels(context.Background(), srv.URL, BackendOllama)
	if err != nil {
		t.Fatalf("DiscoverModels: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestProviderRegistryAllowEmptyKey(t *testing.T) {
	pool := NewPool()
	pool.Register("primary", NewClient("http://example.com", "key", "model"))
	reg := NewProviderRegistry(pool)

	err := reg.Register(ProviderConfig{
		ID:      "local-test",
		Type:    ProviderTypeChat,
		BaseURL: "http://localhost:11434/v1",
		Model:   "llama3",
		Enabled: true,
		// APIKeys intentionally empty — should auto-fill
	})
	if err != nil {
		t.Fatalf("Register with empty APIKeys should succeed: %v", err)
	}
	p := reg.Get("local-test")
	if p == nil {
		t.Fatal("provider not found")
	}
}
