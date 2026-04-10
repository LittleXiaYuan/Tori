package llm

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain detects goroutine leaks across all tests in this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		// ResponseCache.evictLoop: each NewClient starts one; has Stop() but not
		// called in every unit test. Properly stopped in production via Client.Close().
		goleak.IgnoreTopFunction("yunque-agent/internal/agentcore/llm.(*ResponseCache).evictLoop"),
	)
}
