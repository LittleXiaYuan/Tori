package task

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain detects goroutine leaks across all tests in this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("time.Sleep"),
		goleak.IgnoreTopFunction("yunque-agent/internal/agentcore/session.(*Store).gcLoop"),
	)
}
