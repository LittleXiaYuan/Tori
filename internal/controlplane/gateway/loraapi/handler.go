package loraapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/gwshared"
)

// Handler serves LoRA training and evolution HTTP endpoints.
type Handler struct {
	Scheduler   *localbrain.LoRAScheduler
	Metrics     *localbrain.TrainingMetrics
	Evolution   *localbrain.EvolutionCoordinator
}

// RegisterRoutes mounts all /v1/lora/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	mux.HandleFunc("/v1/lora/status", auth(h.handleStatus))
	mux.HandleFunc("/v1/lora/history", auth(h.handleHistory))
	mux.HandleFunc("/v1/lora/summary", auth(h.handleSummary))
	mux.HandleFunc("/v1/lora/trigger", auth(h.handleTrigger))
	mux.HandleFunc("/v1/lora/rollback", auth(h.handleRollback))
	mux.HandleFunc("/v1/lora/evolution", auth(h.handleEvolution))
	mux.HandleFunc("/v1/lora/config", auth(h.handleConfig))
}

func (h *Handler) metrics() *localbrain.TrainingMetrics {
	if h.Metrics != nil {
		return h.Metrics
	}
	if h.Scheduler != nil {
		return h.Scheduler.Metrics()
	}
	return nil
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if h.Scheduler == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "LoRA scheduler not configured")
		return
	}

	out := map[string]any{
		"scheduler":    h.Scheduler.State(),
		"active_model": h.Scheduler.ActiveModel(),
	}
	if h.Evolution != nil {
		out["rolling_success_rate"] = h.Evolution.State().RollingSuccessRate
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	m := h.metrics()
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

func (h *Handler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	m := h.metrics()
	if m == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"summary": struct{}{}})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"summary": m.Summary()})
}

func (h *Handler) handleTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if h.Scheduler == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "LoRA scheduler not configured")
		return
	}

	tenantID := gwshared.TenantFromCtx(r.Context())
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

	err := h.Scheduler.CheckAndTrigger(ctx, tenantID)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "tenant_id": tenantID})
}

func (h *Handler) handleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if h.Scheduler == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "LoRA scheduler not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	err := h.Scheduler.Rollback(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}

func (h *Handler) handleEvolution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if h.Evolution == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "evolution coordinator not configured")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"state": h.Evolution.State()})
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "LoRA scheduler not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"config": h.Scheduler.Config()})

	case http.MethodPut, http.MethodPatch:
		var body struct {
			MinSamples      int     `json:"min_samples"`
			MinInterval     string  `json:"min_interval"`
			EvalMinScore    float64 `json:"eval_min_score"`
			MaxAdapters     int     `json:"max_adapters"`
			BaseModel       string  `json:"base_model"`
			TrainingDataDir string  `json:"training_data_dir"`
			AdapterDir      string  `json:"adapter_dir"`
			ABTestDuration  string  `json:"ab_test_duration"`
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

		h.Scheduler.UpdateConfig(patch)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"config": h.Scheduler.Config(), "status": "updated"})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT/PATCH only")
	}
}
