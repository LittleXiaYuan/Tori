package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/cogni"
)

func (g *Gateway) cogniExperience(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.cogniExperiences == nil {
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}
	es, ok := g.cogniExperiences[id]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"enabled": false, "id": id})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":           id,
		"enabled":      true,
		"summary":      es.Summary(5),
		"stats":        es.Stats(),
		"tool_memory":  es.ToolMemory(""),
		"patterns":     es.Patterns(),
		"domain_facts": es.DomainFacts(),
	})
}

func (g *Gateway) cogniExperienceRecord(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.cogniExperiences == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "experience stores not configured")
		return
	}
	es, ok := g.cogniExperiences[id]
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "no experience store for cogni: "+id)
		return
	}

	var body struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch body.Type {
	case "tool_memory":
		var tm cogni.ToolExperience
		if err := json.Unmarshal(body.Data, &tm); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		es.AddToolMemory(tm)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "type": "tool_memory"})
	case "pattern":
		var p cogni.BehaviorPattern
		if err := json.Unmarshal(body.Data, &p); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		es.SuggestPattern(p)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "type": "pattern", "id": p.ID})
	case "fact":
		var f cogni.DomainFact
		if err := json.Unmarshal(body.Data, &f); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		es.AddFact(f)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "type": "fact"})
	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "type must be tool_memory, pattern, or fact")
	}
}

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
