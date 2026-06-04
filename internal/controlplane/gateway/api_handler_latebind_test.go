package gateway

import (
	"path/filepath"
	"testing"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/connectors"
)

// These tests guard against a regression where the cost / connector / fork
// sub-package HTTP handlers were constructed in routes() with the gateway's
// then-nil dependencies and never re-bound when the real dependencies were
// injected via setters AFTER NewFromConfig. That left /v1/cost/*,
// /v1/connectors/* and /v1/fork/* permanently "not configured" even though the
// underlying subsystems were wired and working. The fix stores the handlers on
// the Gateway and late-binds their exported deps inside the setters.

func TestSetCostTrackerLateBindsCostAPIHandler(t *testing.T) {
	g, _ := newTestGateway()
	if g.costAPIHandler == nil {
		t.Fatal("costAPIHandler should be created during routes()")
	}
	if g.costAPIHandler.Tracker != nil {
		t.Fatalf("expected nil tracker before SetCostTracker, got %v", g.costAPIHandler.Tracker)
	}
	tr := costtrack.New()
	g.SetCostTracker(tr)
	if g.costAPIHandler.Tracker != tr {
		t.Fatal("SetCostTracker did not late-bind costAPIHandler.Tracker — /v1/cost/* would stay 'not configured'")
	}
}

func TestSetConnectorRegistryLateBindsConnectorAPIHandler(t *testing.T) {
	g, _ := newTestGateway()
	if g.connectorAPIHandler == nil {
		t.Fatal("connectorAPIHandler should be created during routes()")
	}
	if g.connectorAPIHandler.Registry != nil {
		t.Fatalf("expected nil registry before SetConnectorRegistry, got %v", g.connectorAPIHandler.Registry)
	}
	reg := connectors.NewRegistry()
	g.SetConnectorRegistry(reg)
	if g.connectorAPIHandler.Registry != reg {
		t.Fatal("SetConnectorRegistry did not late-bind connectorAPIHandler.Registry — /v1/connectors/* would stay 'not configured'")
	}
}

func TestSetForkTreeAndPersisterLateBindForkAPIHandler(t *testing.T) {
	g, _ := newTestGateway()
	if g.forkAPIHandler == nil {
		t.Fatal("forkAPIHandler should be created during routes()")
	}
	if g.forkAPIHandler.ForkTree != nil {
		t.Fatalf("expected nil fork tree before SetForkTree, got %v", g.forkAPIHandler.ForkTree)
	}
	ft := session.NewForkTree()
	g.SetForkTree(ft)
	if g.forkAPIHandler.ForkTree != ft {
		t.Fatal("SetForkTree did not late-bind forkAPIHandler.ForkTree — /v1/fork/* would stay 'not configured'")
	}
	fp := session.NewForkPersister(filepath.Join(t.TempDir(), "forks.json"))
	g.SetForkPersister(fp)
	if g.forkAPIHandler.Persister != fp {
		t.Fatal("SetForkPersister did not late-bind forkAPIHandler.Persister")
	}
}
