package graphpack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/pkg/packruntime"
)

type fakeGW struct{ p *memory.Pipeline }

func (f fakeGW) MemoryPipeline() *memory.Pipeline { return f.p }

// TestGraphPackV2 verifies the graph pack is a v2 Module with the expected route
// surface and degrades gracefully (empty payloads, 200) when the pipeline is not
// configured (native handlers, de-shelled from the gateway).
func TestGraphPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{}) // nil pipeline
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 4 {
		t.Fatalf("Routes len = %d, want 4", got)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec := httptest.NewRecorder()
	h.handleStats(rec, httptest.NewRequest(http.MethodGet, "/v1/graph/stats", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("nil pipeline handleStats = %d, want 200", rec.Code)
	}
	var stats map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats["entities"] != 0 {
		t.Fatalf("nil pipeline entities = %d, want 0", stats["entities"])
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// TestGraphEntityRoundTrip exercises the native POST→GET→stats path end to end
// through the pack handlers, proving the logic moved into the pack (not a shell).
func TestGraphEntityRoundTrip(t *testing.T) {
	h := New(fakeGW{p: memory.NewPipeline(nil, nil)})

	// Create an entity.
	body, _ := json.Marshal(memory.Entity{Name: "云雀", Type: "product"})
	postRec := httptest.NewRecorder()
	h.handleEntities(postRec, httptest.NewRequest(http.MethodPost, "/v1/graph/entities", bytes.NewReader(body)))
	if postRec.Code != http.StatusOK {
		t.Fatalf("post entity = %d, want 200: %s", postRec.Code, postRec.Body.String())
	}

	// Search it back.
	getRec := httptest.NewRecorder()
	h.handleEntities(getRec, httptest.NewRequest(http.MethodGet, "/v1/graph/entities?q=云雀", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get entities = %d, want 200", getRec.Code)
	}
	var resp struct {
		Entities []memory.Entity `json:"entities"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Entities) != 1 || resp.Entities[0].Name != "云雀" {
		t.Fatalf("expected the created entity back, got %s", getRec.Body.String())
	}

	// Stats reflect the new entity.
	statsRec := httptest.NewRecorder()
	h.handleStats(statsRec, httptest.NewRequest(http.MethodGet, "/v1/graph/stats", nil))
	var stats map[string]int
	if err := json.Unmarshal(statsRec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats["entities"] < 1 {
		t.Fatalf("expected entities >= 1, got %d", stats["entities"])
	}
}
