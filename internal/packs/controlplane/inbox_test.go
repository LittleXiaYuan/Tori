package controlplanepack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/observe"
)

type inboxGateway struct {
	store   *inbox.Store
	bridged int
}

func (g *inboxGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *inboxGateway) ApprovalManager() *approval.Manager { return nil }

func (g *inboxGateway) BotManager() *bots.Manager { return nil }

func (g *inboxGateway) InboxStore() *inbox.Store { return g.store }

func (g *inboxGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *inboxGateway) TenantManager() *tenant.Manager { return nil }

func (g *inboxGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *inboxGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *inboxGateway) OutputDir() string { return "" }

func (g *inboxGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *inboxGateway) MetricsPrometheus() string { return "" }

func (g *inboxGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *inboxGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *inboxGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func TestInboxRoutesAreNative(t *testing.T) {
	gateway := &inboxGateway{store: inbox.NewStore(10)}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/v1/inbox", "/v1/inbox/read"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/v1/inbox"](rec, httptest.NewRequest(http.MethodGet, "/v1/inbox", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("inbox route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestInboxPushMarkReadAndDelete(t *testing.T) {
	h := NewHandler(&inboxGateway{store: inbox.NewStore(10)})
	create := httptest.NewRecorder()
	h.handleInbox(create, httptest.NewRequest(http.MethodPost, "/v1/inbox", bytes.NewBufferString(`{"source":"test","content":"hello","action":"trigger"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", create.Code, create.Body.String())
	}
	var item inbox.Item
	if err := json.NewDecoder(create.Body).Decode(&item); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if item.ID == "" || item.Content != "hello" || item.Action != inbox.ActionTrigger {
		t.Fatalf("item = %+v, want trigger item", item)
	}

	read := httptest.NewRecorder()
	h.handleInboxRead(read, httptest.NewRequest(http.MethodPost, "/v1/inbox/read", bytes.NewBufferString(`{"ids":["`+item.ID+`"]}`)))
	if !bytes.Contains(read.Body.Bytes(), []byte(`"marked":1`)) {
		t.Fatalf("expected marked=1, got %s", read.Body.String())
	}

	listUnread := httptest.NewRecorder()
	h.handleInbox(listUnread, httptest.NewRequest(http.MethodGet, "/v1/inbox?unread=true", nil))
	if !bytes.Contains(listUnread.Body.Bytes(), []byte(`"unread":0`)) {
		t.Fatalf("expected unread count 0, got %s", listUnread.Body.String())
	}

	del := httptest.NewRecorder()
	h.handleInbox(del, httptest.NewRequest(http.MethodDelete, "/v1/inbox", bytes.NewBufferString(`{"id":"`+item.ID+`"}`)))
	if del.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200: %s", del.Code, del.Body.String())
	}
}
