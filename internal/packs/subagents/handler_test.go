package subagentspack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/subagent"
)

func TestRoutesMatchSpecs(t *testing.T) {
	h := NewProvider(nil)
	routes := h.Routes()
	specs := RouteSpecs()
	if len(specs) != 4 {
		t.Fatalf("route specs=%d, want 4 method-specific specs", len(specs))
	}
	paths := map[string]bool{}
	for _, route := range routes {
		paths[route.Path] = true
	}
	for _, spec := range specs {
		if !paths[spec.Path] {
			t.Fatalf("route spec path %s has no mounted route", spec.Path)
		}
	}
}

func TestSpawnListAndMessage(t *testing.T) {
	manager := subagent.NewManager()
	h := NewProvider(func() *subagent.Manager { return manager })

	spawnReq := httptest.NewRequest(http.MethodPost, "/v1/subagent", strings.NewReader(`{"parent_id":"p1","name":"researcher","description":"does research","skills":["search"]}`))
	spawnRec := httptest.NewRecorder()
	h.Subagent(spawnRec, spawnReq)
	if spawnRec.Code != http.StatusOK {
		t.Fatalf("spawn status=%d body=%s", spawnRec.Code, spawnRec.Body.String())
	}
	var sa subagent.Subagent
	if err := json.Unmarshal(spawnRec.Body.Bytes(), &sa); err != nil {
		t.Fatalf("decode spawn: %v", err)
	}
	if sa.ID == "" || sa.Name != "researcher" || sa.ParentID != "p1" {
		t.Fatalf("unexpected subagent: %#v", sa)
	}

	msgReq := httptest.NewRequest(http.MethodPost, "/v1/subagent/message", strings.NewReader(`{"id":"`+sa.ID+`","messages":[{"role":"user","content":"hello"}]}`))
	msgRec := httptest.NewRecorder()
	h.Message(msgRec, msgReq)
	if msgRec.Code != http.StatusOK {
		t.Fatalf("message status=%d body=%s", msgRec.Code, msgRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/subagent?parent_id=p1", nil)
	listRec := httptest.NewRecorder()
	h.Subagent(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	var listBody struct {
		Subagents []subagent.Subagent `json:"subagents"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listBody.Subagents) != 1 || len(listBody.Subagents[0].Messages) != 1 {
		t.Fatalf("unexpected list body: %#v", listBody)
	}
}

func TestNilManagerPreservesErrorJSON(t *testing.T) {
	h := NewProvider(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/subagent", nil)
	w := httptest.NewRecorder()

	h.Subagent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "subagent manager not configured" {
		t.Fatalf("unexpected body: %#v", body)
	}
}
