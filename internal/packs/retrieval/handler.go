package retrievalpack

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"

	"yunque-agent/internal/agentcore/embeddings"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.retrieval"

type Gateway interface {
	EmbeddingsResolver() *embeddings.Resolver
	SearchRegistry() *websearch.Registry
	SearchEnabled() bool
}

type Handler struct {
	embeddingsOf func() *embeddings.Resolver
	searchOf     func() *websearch.Registry
	searchOn     func() bool
	host         packruntime.Host
	started      atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil, nil)
	}
	return NewProvider(gateway.EmbeddingsResolver, gateway.SearchRegistry, gateway.SearchEnabled)
}

func NewProvider(
	embeddingsResolver func() *embeddings.Resolver,
	searchRegistry func() *websearch.Registry,
	searchEnabled func() bool,
) *Handler {
	if searchEnabled == nil {
		searchEnabled = func() bool { return true }
	}
	return &Handler{embeddingsOf: embeddingsResolver, searchOf: searchRegistry, searchOn: searchEnabled}
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("retrieval pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/embeddings", Handler: h.Embeddings},
		{Method: http.MethodGet, Path: "/v1/search", Handler: h.Search},
		{Method: http.MethodGet, Path: "/v1/search/providers", Handler: h.SearchProviders},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/embeddings", Description: "List configured embedding providers."},
		{Method: http.MethodPost, Path: "/v1/embeddings", Description: "Generate a vector embedding for text."},
		{Method: http.MethodGet, Path: "/v1/search", Description: "Run a web search through the configured provider."},
		{Method: http.MethodGet, Path: "/v1/search/providers", Description: "List configured web search providers and enablement state."},
	}
}

func Paths() []string {
	return []string{"/v1/embeddings", "/v1/search", "/v1/search/providers"}
}

func (h *Handler) embeddings() *embeddings.Resolver {
	if h.embeddingsOf == nil {
		return nil
	}
	return h.embeddingsOf()
}

func (h *Handler) search() *websearch.Registry {
	if h.searchOf == nil {
		return nil
	}
	return h.searchOf()
}

func (h *Handler) searchEnabled() bool {
	if h.searchOn == nil {
		return true
	}
	return h.searchOn()
}

func (h *Handler) Embeddings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resolver := h.embeddings()
	if resolver == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "embeddings not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{
			"providers": resolver.List(),
		})
	case http.MethodPost:
		var req struct {
			Text     string `json:"text"`
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
			return
		}
		embedder, ok := resolver.Primary()
		if req.Provider != "" {
			embedder, ok = resolver.Get(req.Provider)
		}
		if !ok {
			apperror.WriteCode(w, apperror.CodeBadRequest, "no embedder available")
			return
		}
		vec, err := embedder.Embed(r.Context(), req.Text)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeLLMError, "embedding failed: "+err.Error())
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding":  vec,
			"dimensions": len(vec),
			"model":      embedder.Model(),
		})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if !h.searchEnabled() {
		apperror.WriteCode(w, apperror.CodeBadRequest, "search is disabled")
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "q is required")
		return
	}
	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	registry := h.search()
	if registry == nil || len(registry.List()) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results":   []websearch.Result{},
			"total":     0,
			"enabled":   false,
			"providers": []string{},
		})
		return
	}
	provider := r.URL.Query().Get("provider")
	var results any
	var err error
	if provider != "" {
		results, err = registry.SearchWith(r.Context(), provider, query, limit)
	} else {
		results, err = registry.Search(r.Context(), query, limit)
	}
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "search failed", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"results": results,
	})
}

func (h *Handler) SearchProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	registry := h.search()
	if registry == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false, "providers": []string{}})
		return
	}
	providers := registry.List()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled":   h.searchEnabled() && len(providers) > 0,
		"providers": providers,
	})
}
