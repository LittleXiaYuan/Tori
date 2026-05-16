package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/workflow"
)

func TestWorkflowRoutesUseLateBoundStore(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("workflow-late-binding")

	store := workflow.NewJSONStore(t.TempDir())
	gw.SetWorkflowStore(store)

	body := bytes.NewBufferString(`{"requirement":"打开小红书创作中心，生成一条效率演示笔记，填写标题和正文，截图留痕后直接点击发布。"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/generate", body)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected generated workflow after late store binding, got status=%d body=%s", w.Code, w.Body.String())
	}

	var res struct {
		OK       bool                 `json:"ok"`
		Workflow *workflow.Definition `json:"workflow"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !res.OK || res.Workflow == nil {
		t.Fatalf("missing workflow in response: %#v", res)
	}

	foundPublishNode := false
	for _, node := range res.Workflow.Nodes {
		if node.ID == "publish" && node.Config["text_target"] == "发布" {
			foundPublishNode = true
			break
		}
	}
	if !foundPublishNode {
		t.Fatalf("expected direct publish browser node, got nodes=%#v", res.Workflow.Nodes)
	}
}
