package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"yunque-agent/internal/agentcore/persona"
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

func (g *Gateway) handlePresets(w http.ResponseWriter, r *http.Request) {
	if g.personaChain == nil || g.personaChain.Presets() == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"presets": []any{}, "active": ""})
		return
	}
	pm := g.personaChain.Presets()

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"presets": pm.List(),
			"active":  pm.ActiveID(),
		})

	case http.MethodPost:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
			return
		}
		if err := pm.Switch(req.ID); err != nil {
			http.Error(w, `{"error":"preset not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "active": req.ID})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (g *Gateway) handlePresetFeatures(w http.ResponseWriter, r *http.Request) {
	if g.personaChain == nil || g.personaChain.Presets() == nil {
		http.Error(w, `{"error":"presets not configured"}`, http.StatusNotFound)
		return
	}
	pm := g.personaChain.Presets()

	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       string          `json:"id"`
		Features map[string]bool `json:"features"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, `{"error":"id and features required"}`, http.StatusBadRequest)
		return
	}
	if err := pm.SetFeatures(req.ID, req.Features); err != nil {
		http.Error(w, `{"error":"preset not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleCustomPreset(w http.ResponseWriter, r *http.Request) {
	if g.personaChain == nil || g.personaChain.Presets() == nil {
		http.Error(w, `{"error":"presets not configured"}`, http.StatusNotFound)
		return
	}
	pm := g.personaChain.Presets()

	switch r.Method {
	case http.MethodPost:
		var req struct {
			ID          string          `json:"id"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Tone        string          `json:"tone"`
			Style       string          `json:"style"`
			Greeting    string          `json:"greeting"`
			SystemNote  string          `json:"system_note"`
			Features    map[string]bool `json:"features,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" || req.Name == "" {
			http.Error(w, `{"error":"id and name required"}`, http.StatusBadRequest)
			return
		}
		p := persona.Preset{
			ID:          req.ID,
			Name:        req.Name,
			Description: req.Description,
			Tone:        req.Tone,
			Style:       req.Style,
			Greeting:    req.Greeting,
			SystemNote:  req.SystemNote,
			Features:    req.Features,
		}
		pm.AddCustom(p)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": req.ID})

	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
			return
		}
		if err := pm.RemoveCustom(req.ID); err != nil {
			http.Error(w, `{"error":"not found or not custom"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
