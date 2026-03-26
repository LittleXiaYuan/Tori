// Package opp provides data structures and protocol logic for the Open Plugin Protocol.
//
// OPP is a protocol for agent-to-agent task delegation with typed intents,
// a deterministic state machine, and interactive mid-task negotiation
// (QUESTION/ANSWER, PROBLEM/DECIDE).
//
// This package handles serialization, validation, and state transitions only.
// It does not provide any network transport, session management, or runtime execution.
package opp
