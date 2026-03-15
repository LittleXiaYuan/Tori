package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Embedder produces vector embeddings from text.
type Embedder interface {
	Embed(ctx context.Context, input string) ([]float32, error)
	EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error)
	Dimensions() int
	Model() string
}

// Resolver manages multiple embedders and selects the appropriate one.
type Resolver struct {
	mu        sync.RWMutex
	embedders map[string]Embedder
	primary   string
}

// NewResolver creates an embeddings resolver.
func NewResolver() *Resolver {
	return &Resolver{embedders: make(map[string]Embedder)}
}

// Register adds an embedder with a name.
func (r *Resolver) Register(name string, e Embedder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.embedders[name] = e
	if r.primary == "" {
		r.primary = name
	}
}

// SetPrimary sets the default embedder.
func (r *Resolver) SetPrimary(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.embedders[name]; !ok {
		return false
	}
	r.primary = name
	return true
}

// Get returns a named embedder.
func (r *Resolver) Get(name string) (Embedder, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.embedders[name]
	return e, ok
}

// Primary returns the default embedder.
func (r *Resolver) Primary() (Embedder, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.primary == "" {
		return nil, false
	}
	e, ok := r.embedders[r.primary]
	return e, ok
}

// List returns all registered embedder names.
func (r *Resolver) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.embedders))
	for n := range r.embedders {
		names = append(names, n)
	}
	return names
}

// --- OpenAI-compatible embedder ---

type openAIRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openAIResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// OpenAIEmbedder calls OpenAI-compatible embedding APIs.
type OpenAIEmbedder struct {
	apiKey  string
	baseURL string
	model   string
	dims    int
	client  *http.Client
	logger  *slog.Logger
}

// NewOpenAI creates an OpenAI-compatible embedder.
func NewOpenAI(apiKey, baseURL, model string, dims int) (*OpenAIEmbedder, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("embedder: base url required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("embedder: model required")
	}
	if dims <= 0 {
		dims = 1536
	}
	return &OpenAIEmbedder{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		dims:    dims,
		client:  &http.Client{Timeout: 30 * time.Second},
		logger:  slog.Default().With(slog.String("embedder", model)),
	}, nil
}

func (e *OpenAIEmbedder) Model() string      { return e.model }
func (e *OpenAIEmbedder) Dimensions() int     { return e.dims }

func (e *OpenAIEmbedder) Embed(ctx context.Context, input string) ([]float32, error) {
	vecs, err := e.EmbedBatch(ctx, []string{input})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return vecs[0], nil
}

func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(openAIRequest{Input: inputs, Model: e.model})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	result := make([][]float32, len(inputs))
	for _, d := range parsed.Data {
		if d.Index < len(result) {
			result[d.Index] = d.Embedding
		}
	}
	return result, nil
}

// --- Similarity functions ---

// CosineSimilarity computes cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// TopK returns indices of the K most similar vectors to query.
type ScoredIndex struct {
	Index int
	Score float64
}

func TopK(query []float32, corpus [][]float32, k int) []ScoredIndex {
	if k <= 0 || len(corpus) == 0 {
		return nil
	}
	scores := make([]ScoredIndex, len(corpus))
	for i, vec := range corpus {
		scores[i] = ScoredIndex{Index: i, Score: CosineSimilarity(query, vec)}
	}
	// Simple selection sort for top-k (sufficient for typical corpus sizes)
	for i := 0; i < k && i < len(scores); i++ {
		best := i
		for j := i + 1; j < len(scores); j++ {
			if scores[j].Score > scores[best].Score {
				best = j
			}
		}
		scores[i], scores[best] = scores[best], scores[i]
	}
	if k > len(scores) {
		k = len(scores)
	}
	return scores[:k]
}
