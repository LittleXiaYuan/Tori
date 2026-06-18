package marketpack

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"

	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.market"

type Gateway interface {
	SkillMarket() *skillmarket.Market
}

// Handler exposes the local skill market catalog as a native capability pack.
type Handler struct {
	marketOf func() *skillmarket.Market
	host     packruntime.Host
	started  atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil)
	}
	return NewProvider(gateway.SkillMarket)
}

func NewProvider(market func() *skillmarket.Market) *Handler {
	return &Handler{marketOf: market}
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
		h.host.Logger().Info("market pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/market/search", Handler: h.Search},
		{Method: http.MethodGet, Path: "/v1/market/top", Handler: h.Top},
		{Method: http.MethodGet, Path: "/v1/market/stats", Handler: h.Stats},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/market/search", Description: "Search the local skill market catalog."},
		{Method: http.MethodGet, Path: "/v1/market/top", Description: "List top local skill market entries by popularity or rating."},
		{Method: http.MethodGet, Path: "/v1/market/stats", Description: "Return aggregate local skill market statistics."},
	}
}

func Paths() []string {
	return []string{"/v1/market/search", "/v1/market/top", "/v1/market/stats"}
}

func (h *Handler) market() *skillmarket.Market {
	if h.marketOf == nil {
		return nil
	}
	return h.marketOf()
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	market := h.market()
	if market == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "skill market not configured"})
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		_ = json.NewEncoder(w).Encode(map[string]any{"skills": market.All()})
		return
	}
	results := market.Search(query)
	_ = json.NewEncoder(w).Encode(map[string]any{"skills": results, "count": len(results)})
}

func (h *Handler) Top(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	market := h.market()
	if market == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "skill market not configured"})
		return
	}
	n := 10
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	if r.URL.Query().Get("by") == "rating" {
		_ = json.NewEncoder(w).Encode(map[string]any{"skills": market.TopRated(n)})
	} else {
		_ = json.NewEncoder(w).Encode(map[string]any{"skills": market.MostPopular(n)})
	}
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	market := h.market()
	if market == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "skill market not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(market.Stats())
}
