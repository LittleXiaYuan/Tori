package workflowapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/workflow"
)

func noAuth(next http.HandlerFunc) http.HandlerFunc { return next }

func TestGenerateEndpointSavesTemplateWorkflow(t *testing.T) {
	store := workflow.NewJSONStore(t.TempDir())
	h := &Handler{Store: store}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, noAuth)

	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/generate", bytes.NewBufferString(`{"requirement":"每天早上生成项目日报"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		OK          bool                 `json:"ok"`
		GeneratedBy string               `json:"generated_by"`
		Workflow    *workflow.Definition `json:"workflow"`
		Message     string               `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.GeneratedBy != string(workflow.GenerationSourceTemplate) {
		t.Fatalf("unexpected body: %#v", body)
	}
	if body.Workflow == nil || body.Workflow.ID == "" {
		t.Fatalf("missing workflow in body: %#v", body)
	}
	got, err := store.GetDefinition(body.Workflow.ID)
	if err != nil {
		t.Fatalf("generated workflow should be saved: %v", err)
	}
	if got.Name == "" || len(got.Nodes) == 0 {
		t.Fatalf("saved workflow incomplete: %#v", got)
	}
}
