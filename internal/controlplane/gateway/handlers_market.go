package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (g *Gateway) handleMarketSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillMarket == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "skill market not configured"})
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode(map[string]any{"skills": g.skillMarket.All()})
		return
	}
	results := g.skillMarket.Search(query)
	json.NewEncoder(w).Encode(map[string]any{"skills": results, "count": len(results)})
}

func (g *Gateway) handleMarketTop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillMarket == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "skill market not configured"})
		return
	}
	n := 10
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	by := r.URL.Query().Get("by")
	if by == "rating" {
		json.NewEncoder(w).Encode(map[string]any{"skills": g.skillMarket.TopRated(n)})
	} else {
		json.NewEncoder(w).Encode(map[string]any{"skills": g.skillMarket.MostPopular(n)})
	}
}

func (g *Gateway) handleMarketStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.skillMarket == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "skill market not configured"})
		return
	}
	json.NewEncoder(w).Encode(g.skillMarket.Stats())
}
