package gateway

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain detects goroutine leaks across all tests in this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Known background goroutines from infrastructure components
		goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("time.Sleep"),
		// session.Store GC loop — long-lived by design
		goleak.IgnoreTopFunction("yunque-agent/internal/agentcore/session.(*Store).gcLoop"),
		// ResponseCache eviction — long-lived by design
		goleak.IgnoreTopFunction("yunque-agent/internal/agentcore/llm.(*ResponseCache).evictLoop"),
		// trigger.Runtime condition loop — long-lived by design
		goleak.IgnoreTopFunction("yunque-agent/internal/agentcore/trigger.(*Runtime).conditionLoop"),
		// RateLimiter cleanup — long-lived by design
		goleak.IgnoreTopFunction("yunque-agent/internal/controlplane/gateway.(*RateLimiter).cleanup"),
		// MCP WorkerRegistry reaper — long-lived by design (registered via
		// registerMCPDispatchRoutes inside gateway.New); a Stop() method
		// exists but is owned by the lifecycle layer in cmd/agent.
		goleak.IgnoreTopFunction("yunque-agent/internal/mcp/server.(*WorkerRegistry).reapLoop"),
	)
}
