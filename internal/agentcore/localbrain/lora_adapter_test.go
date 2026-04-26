package localbrain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewLoRAAdapter(t *testing.T) {
	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://localhost:8000"})
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
	if a.baseURL != "http://localhost:8000" {
		t.Errorf("baseURL = %s", a.baseURL)
	}
}

func TestNewLoRAAdapter_CustomTimeout(t *testing.T) {
	a := NewLoRAAdapter(LoRAAdapterConfig{
		BaseURL: "http://localhost:8000",
		Timeout: 5 * time.Second,
	})
	if a.httpClient.Timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", a.httpClient.Timeout)
	}
}

func TestModelName(t *testing.T) {
	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://localhost:8000"})

	tests := []struct {
		base, lora, want string
	}{
		{"qwen-7b", "finance-v1", "qwen-7b:finance-v1"},
		{"qwen-7b", "", "qwen-7b"},
	}
	for _, tt := range tests {
		got := a.ModelName(tt.base, tt.lora)
		if got != tt.want {
			t.Errorf("ModelName(%s, %s) = %s, want %s", tt.base, tt.lora, got, tt.want)
		}
	}
}

func TestIsLoaded(t *testing.T) {
	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://localhost:8000"})

	if a.IsLoaded("nonexistent") {
		t.Error("expected false for unknown adapter")
	}

	a.knownAdapters["test"] = &AdapterInfo{Name: "test", Status: "loaded"}
	if !a.IsLoaded("test") {
		t.Error("expected true for loaded adapter")
	}

	a.knownAdapters["err"] = &AdapterInfo{Name: "err", Status: "error"}
	if a.IsLoaded("err") {
		t.Error("expected false for error-status adapter")
	}
}

func TestLoad_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/lora/load" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["lora_name"] != "adapter-v1" {
			t.Errorf("lora_name = %s", body["lora_name"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL})
	err := a.Load(context.Background(), "adapter-v1", "/weights/v1", "qwen-7b")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !a.IsLoaded("adapter-v1") {
		t.Error("adapter should be loaded after Load()")
	}
}

func TestLoad_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("gpu out of memory"))
	}))
	defer srv.Close()

	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL})
	err := a.Load(context.Background(), "bad-adapter", "/weights/bad", "qwen-7b")
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestLoad_ServerUnreachable(t *testing.T) {
	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://127.0.0.1:1"})
	err := a.Load(context.Background(), "offline", "/weights/x", "qwen-7b")
	if err != nil {
		t.Fatal("unreachable server should fall back to local registry, not error")
	}
	info, ok := a.knownAdapters["offline"]
	if !ok {
		t.Fatal("adapter should be registered locally on connection failure")
	}
	if info.Status != "local_only" {
		t.Errorf("status = %s, want local_only for unreachable server", info.Status)
	}
	if a.IsLoaded("offline") {
		t.Error("IsLoaded should return false for local_only status")
	}
}

func TestUnload_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL})
	a.knownAdapters["v1"] = &AdapterInfo{Name: "v1", Status: "loaded"}

	err := a.Unload(context.Background(), "v1")
	if err != nil {
		t.Fatalf("Unload failed: %v", err)
	}
	if a.IsLoaded("v1") {
		t.Error("adapter should be removed after Unload()")
	}
}

func TestList_FromServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/lora/list" {
			t.Errorf("path = %s", r.URL.Path)
		}
		resp := struct {
			Adapters []*AdapterInfo `json:"adapters"`
		}{
			Adapters: []*AdapterInfo{
				{Name: "a1", Status: "loaded", BaseModel: "qwen-7b"},
				{Name: "a2", Status: "loaded", BaseModel: "qwen-7b"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL})
	list, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestList_FallbackToLocal(t *testing.T) {
	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://127.0.0.1:1"})
	a.knownAdapters["local1"] = &AdapterInfo{Name: "local1", Status: "loaded"}

	list, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len = %d, want 1", len(list))
	}
}

func TestLoad_WithAPIKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL, APIKey: "sk-test-123"})
	a.Load(context.Background(), "x", "/w", "m")

	if gotAuth != "Bearer sk-test-123" {
		t.Errorf("Authorization = %s, want Bearer sk-test-123", gotAuth)
	}
}

func TestUnload_ServerUnreachable(t *testing.T) {
	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://127.0.0.1:1"})
	a.knownAdapters["v1"] = &AdapterInfo{Name: "v1", Status: "loaded"}

	err := a.Unload(context.Background(), "v1")
	if err != nil {
		t.Fatalf("unreachable Unload should not error: %v", err)
	}
	if _, ok := a.knownAdapters["v1"]; ok {
		t.Error("adapter should be removed from local registry even if server unreachable")
	}
}

func TestList_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL})
	a.knownAdapters["local"] = &AdapterInfo{Name: "local", Status: "loaded"}

	list, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List should not error on 500: %v", err)
	}
	if len(list) != 1 || list[0].Name != "local" {
		t.Errorf("should fallback to local list, got %d items", len(list))
	}
}

func TestList_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL})
	a.knownAdapters["x"] = &AdapterInfo{Name: "x", Status: "loaded"}

	list, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List should not error on bad JSON: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("should fallback to local, got %d", len(list))
	}
}

func TestLoad_MultipleAdapters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: srv.URL})
	for _, name := range []string{"a1", "a2", "a3"} {
		if err := a.Load(context.Background(), name, "/w/"+name, "base"); err != nil {
			t.Fatalf("Load %s: %v", name, err)
		}
	}

	list, _ := a.List(context.Background())
	if len(list) < 3 {
		t.Errorf("expected at least 3 adapters, got %d", len(list))
	}
}

func TestNewLoRAAdapter_DefaultTimeout(t *testing.T) {
	a := NewLoRAAdapter(LoRAAdapterConfig{BaseURL: "http://localhost:8000"})
	if a.httpClient.Timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", a.httpClient.Timeout)
	}
}
