package retrievalpack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/embeddings"
	"yunque-agent/internal/agentcore/websearch"
)

type testEmbedder struct{}

func (testEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (testEmbedder) EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error) {
	out := make([][]float32, 0, len(inputs))
	for range inputs {
		vec, err := (testEmbedder{}).Embed(ctx, "")
		if err != nil {
			return nil, err
		}
		out = append(out, vec)
	}
	return out, nil
}

func (testEmbedder) Dimensions() int { return 3 }
func (testEmbedder) Model() string   { return "test-embed" }

type testSearchProvider struct{}

func (testSearchProvider) Name() string { return "test" }
func (testSearchProvider) Search(context.Context, string, int) ([]websearch.Result, error) {
	return []websearch.Result{{Title: "Hello", URL: "https://example.test", Snippet: "world"}}, nil
}

func TestRoutesMatchSpecs(t *testing.T) {
	h := NewProvider(nil, nil, nil)
	routes := h.Routes()
	specs := RouteSpecs()
	if len(specs) != 4 {
		t.Fatalf("route specs=%d, want 4 method-specific specs", len(specs))
	}
	paths := map[string]bool{}
	for _, route := range routes {
		paths[route.Path] = true
	}
	for _, spec := range specs {
		if !paths[spec.Path] {
			t.Fatalf("route spec path %s has no mounted route", spec.Path)
		}
	}
}

func TestEmbeddingsPost(t *testing.T) {
	resolver := embeddings.NewResolver()
	resolver.Register("test", testEmbedder{})
	h := NewProvider(func() *embeddings.Resolver { return resolver }, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", body(`{"text":"hello"}`))
	w := httptest.NewRecorder()

	h.Embeddings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Embedding  []float32 `json:"embedding"`
		Dimensions int       `json:"dimensions"`
		Model      string    `json:"model"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Dimensions != 3 || got.Model != "test-embed" || len(got.Embedding) != 3 {
		t.Fatalf("unexpected embedding response: %#v", got)
	}
}

func TestSearchNoProvidersPreservesEmptyResponse(t *testing.T) {
	registry := websearch.NewRegistry()
	h := NewProvider(nil, func() *websearch.Registry { return registry }, func() bool { return true })
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&limit=2", nil)
	w := httptest.NewRecorder()

	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Results   []websearch.Result `json:"results"`
		Total     int                `json:"total"`
		Enabled   bool               `json:"enabled"`
		Providers []string           `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Results) != 0 || got.Total != 0 || got.Enabled || len(got.Providers) != 0 {
		t.Fatalf("unexpected empty response: %#v", got)
	}
}

func TestSearchProviders(t *testing.T) {
	registry := websearch.NewRegistry()
	registry.Register(testSearchProvider{})
	h := NewProvider(nil, func() *websearch.Registry { return registry }, func() bool { return true })
	req := httptest.NewRequest(http.MethodGet, "/v1/search/providers", nil)
	w := httptest.NewRecorder()

	h.SearchProviders(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Enabled   bool     `json:"enabled"`
		Providers []string `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Enabled || len(got.Providers) != 1 || got.Providers[0] != "test" {
		t.Fatalf("unexpected providers response: %#v", got)
	}
}

func body(s string) *strings.Reader { return strings.NewReader(s) }
