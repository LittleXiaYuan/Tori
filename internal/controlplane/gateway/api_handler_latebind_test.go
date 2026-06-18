package gateway

import (
	"path/filepath"
	"testing"

	"yunque-agent/internal/agentcore/session"
)

// These tests guard against a regression where the fork sub-package HTTP
// handler was constructed in routes() with the gateway's then-nil dependencies
// and never re-bound when the real dependencies were injected via setters AFTER
// NewFromConfig. That left /v1/fork/* permanently "not configured" even though
// the underlying subsystem was wired and working. Cost and connector routes
// moved to native packs and are covered by migration pack tests.

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
