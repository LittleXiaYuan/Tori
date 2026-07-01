package connectors

import (
	"context"
	"testing"
)

type fakeRegistryConnectorHandler struct {
	failExecute bool
}

func (h *fakeRegistryConnectorHandler) Connect(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *fakeRegistryConnectorHandler) Disconnect(_ context.Context, _ *Credentials) error {
	return nil
}

func (h *fakeRegistryConnectorHandler) Refresh(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *fakeRegistryConnectorHandler) Validate(_ context.Context, _ *Credentials) (bool, string, error) {
	return true, "demo-user", nil
}

func (h *fakeRegistryConnectorHandler) Execute(_ context.Context, _ *Credentials, actionID string, _ map[string]any) (any, error) {
	if h.failExecute {
		return nil, errFakeConnectorExecute
	}
	return map[string]any{"action": actionID}, nil
}

var errFakeConnectorExecute = &fakeConnectorError{"upstream denied"}

type fakeConnectorError struct {
	message string
}

func (e *fakeConnectorError) Error() string { return e.message }

func TestRegistryTracksConnectorLastEvent(t *testing.T) {
	reg := NewRegistry()
	reg.store = t.TempDir()
	handler := &fakeRegistryConnectorHandler{}
	reg.RegisterDef(&ConnectorDef{
		ID:       "demo",
		Name:     "Demo",
		AuthType: "token",
		Actions:  []ActionDef{{ID: "ping", Name: "Ping"}},
	})
	reg.RegisterHandler("demo", handler)

	if err := reg.ConnectWithKey(context.Background(), "demo", "secret"); err != nil {
		t.Fatalf("connect: %v", err)
	}
	inst := reg.GetInstance("demo")
	if inst.LastEvent == nil || inst.LastEvent.Kind != "connect" || inst.LastEvent.Status != "ok" {
		t.Fatalf("expected connect event, got %+v", inst.LastEvent)
	}

	if _, err := reg.Execute(context.Background(), "demo", "ping", nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	inst = reg.GetInstance("demo")
	if inst.LastEvent == nil || inst.LastEvent.Kind != "execute" || inst.LastEvent.ActionID != "ping" || inst.LastEvent.Status != "ok" {
		t.Fatalf("expected execute success event, got %+v", inst.LastEvent)
	}

	handler.failExecute = true
	if _, err := reg.Execute(context.Background(), "demo", "ping", nil); err == nil {
		t.Fatal("expected execute failure")
	}
	inst = reg.GetInstance("demo")
	if inst.LastEvent == nil || inst.LastEvent.Kind != "execute" || inst.LastEvent.Status != "error" || inst.LastEvent.Message == "" {
		t.Fatalf("expected execute error event, got %+v", inst.LastEvent)
	}
}
