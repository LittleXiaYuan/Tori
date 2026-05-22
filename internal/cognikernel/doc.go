// Package cognikernel owns the canonical AgentCore cognitive loops.
//
// In particular, ReflectiveLoop is the canonical post-turn reflection and
// learning pipeline. New reflection features should enter through
// ReflectiveLoop hooks or kernel events, not through planner-side background
// cognition or the legacy internal/experimental/reflect package.
package cognikernel
