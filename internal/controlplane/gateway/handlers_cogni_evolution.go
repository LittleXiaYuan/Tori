package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/apperror"
)

func (g *Gateway) cogniEvolve(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniEvolution == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "evolution engine not configured")
		return
	}
	if g.cogniEvolution.IsRunning(id) {
		apperror.WriteCode(w, apperror.CodeBadRequest, "evolution already running for "+id)
		return
	}

	go func() {
		exps, err := g.cogniEvolution.Run(context.Background(), id)
		if err != nil {
			slog.Error("evolution failed", "cogni", id, "err", err)
		} else {
			slog.Info("evolution complete", "cogni", id, "experiments", len(exps))
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "started",
		"id":     id,
	})
}

func (g *Gateway) cogniEvolutionByID(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniEvolution == nil {
		json.NewEncoder(w).Encode(map[string]any{"experiments": []any{}, "running": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"id":          id,
		"experiments": g.cogniEvolution.Experiments(id),
		"running":     g.cogniEvolution.IsRunning(id),
	})
}

func (g *Gateway) cogniEvolutionList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.cogniEvolution == nil {
		json.NewEncoder(w).Encode(map[string]any{"experiments": map[string]any{}})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"experiments": g.cogniEvolution.AllExperiments(),
	})
}
