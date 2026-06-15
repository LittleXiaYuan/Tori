package instructionspack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/instruction"
	"yunque-agent/internal/controlplane/gateway/gwshared"
	"yunque-agent/pkg/packruntime"
)

// memKV is an in-memory instruction.KV for tests: it round-trips values through
// JSON so it exercises the same marshal/unmarshal path as the real KV store.
type memKV struct{ m map[string][]byte }

func newMemKV() *memKV { return &memKV{m: map[string][]byte{}} }

func (k *memKV) Get(_ context.Context, key string, dest any) (bool, error) {
	b, ok := k.m[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(b, dest)
}

func (k *memKV) Put(_ context.Context, key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	k.m[key] = b
	return nil
}

type fakeGW struct{ store *instruction.Store }

func (f fakeGW) InstructionStore() *instruction.Store { return f.store }

// TestInstructionsPackV2 verifies the instructions pack is a v2 Module with the
// expected route surface and degrades to 404 when the store is not configured
// (native handler, de-shelled from the gateway).
func TestInstructionsPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{}) // nil store
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 2 {
		t.Fatalf("Routes len = %d, want 2", got)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec := httptest.NewRecorder()
	h.handleInstructions(rec, httptest.NewRequest(http.MethodGet, "/v1/instructions", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("nil store handleInstructions = %d, want 404", rec.Code)
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// TestInstructionsCRUDRoundTrip exercises the native create→list path end to
// end through the pack handler, proving the logic moved into the pack (not a
// shell that forwards to the gateway).
func TestInstructionsCRUDRoundTrip(t *testing.T) {
	store := instruction.NewStore(newMemKV())
	h := New(fakeGW{store: store})
	ctx := gwshared.ContextWithTenant(context.Background(), "tenant-a")

	// Create
	body, _ := json.Marshal(instruction.UserInstruction{Content: "始终用中文回复", Category: "tone"})
	createReq := httptest.NewRequest(http.MethodPost, "/v1/instructions", bytes.NewReader(body)).WithContext(ctx)
	createRec := httptest.NewRecorder()
	h.handleInstructions(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201: %s", createRec.Code, createRec.Body.String())
	}

	// List
	listReq := httptest.NewRequest(http.MethodGet, "/v1/instructions", nil).WithContext(ctx)
	listRec := httptest.NewRecorder()
	h.handleInstructions(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list = %d, want 200", listRec.Code)
	}
	var resp struct {
		Instructions []instruction.UserInstruction `json:"instructions"`
		Total        int                           `json:"total"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Instructions) != 1 {
		t.Fatalf("expected 1 instruction, got total=%d len=%d", resp.Total, len(resp.Instructions))
	}
	if resp.Instructions[0].Content != "始终用中文回复" {
		t.Fatalf("unexpected content: %q", resp.Instructions[0].Content)
	}

	// Tenant isolation: a different tenant sees nothing.
	otherCtx := gwshared.ContextWithTenant(context.Background(), "tenant-b")
	otherRec := httptest.NewRecorder()
	h.handleInstructions(otherRec, httptest.NewRequest(http.MethodGet, "/v1/instructions", nil).WithContext(otherCtx))
	var otherResp struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(otherRec.Body.Bytes(), &otherResp)
	if otherResp.Total != 0 {
		t.Fatalf("tenant-b should see 0 instructions, got %d", otherResp.Total)
	}
}
