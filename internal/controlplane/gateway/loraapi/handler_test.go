package loraapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/controlplane/gateway/gwshared"
)

func TestHandlePreviewReturnsTrainingReadiness(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "train.jsonl"), []byte(
		"{\"instruction\":\"hello\",\"input\":\"world\",\"output\":\"useful output text\"}\n",
	), 0644); err != nil {
		t.Fatalf("write training data: %v", err)
	}

	cfg := localbrain.DefaultSchedulerConfig()
	cfg.TrainingDataDir = dataDir
	cfg.MinSamples = 1
	scheduler := localbrain.NewLoRAScheduler(nil, nil, nil, cfg)

	mux := http.NewServeMux()
	handler := &Handler{Scheduler: scheduler}
	handler.RegisterRoutes(mux, func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			next(w, r.WithContext(gwshared.ContextWithTenant(r.Context(), "tenant-a")))
		}
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/lora/preview", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Preview localbrain.TrainingDataPreview `json:"preview"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Preview.Ready {
		t.Fatalf("preview should be ready: %+v", body.Preview)
	}
	if body.Preview.TenantID != "tenant-a" {
		t.Fatalf("tenant_id = %q, want tenant-a", body.Preview.TenantID)
	}
}
