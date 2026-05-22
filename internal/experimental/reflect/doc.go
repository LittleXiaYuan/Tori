// Package reflect contains the legacy reflection evaluator and learning loop.
//
// Deprecated: internal/cognikernel.ReflectiveLoop is the canonical AgentCore
// post-turn reflection owner. Existing callers may keep using this package
// through Engine.AsReflectEvalFunc while they migrate to the cognikernel
// pipeline. New reflection features must be implemented as ReflectiveLoop
// hooks, kernel events, or adapters feeding ReflectiveLoop instead of writing
// new owner logic here.
package reflect
