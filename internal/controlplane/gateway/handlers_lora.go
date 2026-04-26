package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/apperror"
)

func (g *Gateway) handleLoRAStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.loraScheduler == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "LoRA scheduler not configured")
		return
	}

	state := g.loraScheduler.State()
	active := g.loraScheduler.ActiveModel()

	out := map[string]any{
		"scheduler":    state,
		"active_model": active,
	}
	if g.evolutionCoordinator != nil {
		out["rolling_success_rate"] = g.evolutionCoordinator.State().RollingSuccessRate
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (g *Gateway) handleLoRAHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	m := g.trainingMetrics
	if m == nil && g.loraScheduler != nil {
		m = g.loraScheduler.Metrics()
	}
	if m == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"records": []any{}, "count": 0})
		return
	}

	recs := m.Recent(50)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"records": recs,
		"count":   len(recs),
	})
}

func (g *Gateway) handleLoRASummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	m := g.trainingMetrics
	if m == nil && g.loraScheduler != nil {
		m = g.loraScheduler.Metrics()
	}
	if m == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"summary": struct{}{}})
		return
	}

	summary := m.Summary()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"summary": summary})
}

func (g *Gateway) handleLoRATrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.loraScheduler == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "LoRA scheduler not configured")
		return
	}

	tenantID := tenantFromCtx(r.Context())
	if tenantID == "" || tenantID == "setup" {
		tenantID = "default"
	}

	var body struct {
		TenantID string `json:"tenant_id"`
	}
	if r.Body != nil {
		data, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		_ = r.Body.Close()
		if len(data) > 0 {
			_ = json.Unmarshal(data, &body)
		}
	}
	if body.TenantID != "" {
		tenantID = body.TenantID
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Hour)
	defer cancel()

	err := g.loraScheduler.CheckAndTrigger(ctx, tenantID)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "tenant_id": tenantID})
}

func (g *Gateway) handleLoRARollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.loraScheduler == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "LoRA scheduler not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	err := g.loraScheduler.Rollback(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}

func (g *Gateway) handleLoRAConfig(w http.ResponseWriter, r *http.Request) {
	if g.loraScheduler == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "LoRA scheduler not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg := g.loraScheduler.Config()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"config": cfg})

	case http.MethodPut, http.MethodPatch:
		var body struct {
			MinSamples     int    `json:"min_samples"`
			MinInterval    string `json:"min_interval"`
			EvalMinScore   float64 `json:"eval_min_score"`
			MaxAdapters    int    `json:"max_adapters"`
			BaseModel      string `json:"base_model"`
			TrainingDataDir string `json:"training_data_dir"`
			AdapterDir     string `json:"adapter_dir"`
			ABTestDuration string `json:"ab_test_duration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
			return
		}

		var patch localbrain.SchedulerConfig
		patch.MinSamples = body.MinSamples
		if body.MinInterval != "" {
			if d, err := time.ParseDuration(body.MinInterval); err == nil {
				patch.MinInterval = d
			}
		}
		patch.EvalMinScore = body.EvalMinScore
		patch.MaxAdapters = body.MaxAdapters
		patch.BaseModel = body.BaseModel
		patch.TrainingDataDir = body.TrainingDataDir
		patch.AdapterDir = body.AdapterDir
		if body.ABTestDuration != "" {
			if d, err := time.ParseDuration(body.ABTestDuration); err == nil {
				patch.ABTestDuration = d
			}
		}

		g.loraScheduler.UpdateConfig(patch)
		cfg := g.loraScheduler.Config()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"config": cfg, "status": "updated"})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT/PATCH only")
	}
}

func (g *Gateway) handleLoRAEvolution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.evolutionCoordinator == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "evolution coordinator not configured")
		return
	}

	state := g.evolutionCoordinator.State()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"state": state})
}
