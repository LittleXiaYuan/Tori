// This example demonstrates a complete OPP task lifecycle:
// Intent → Accept → Progress → Problem/Decide → Result
//
// NOTE: This file is meant for the standalone opp-go repository.
// When used inside the monorepo, change the import to "yunque-agent/pkg/opp".
package main

import (
	"fmt"

	opp "yunque-agent/pkg/opp"
)

func main() {
	const (
		caller = "ide-agent"
		worker = "deploy-agent"
		sess   = "session-001"
		task   = "task-001"
	)

	// Step 1: Caller sends an intent
	intentMsg := opp.NewIntent(caller, worker, sess, opp.IntentEnvelope{
		Name:    "ops.deploy",
		Version: "1.0",
		Payload: map[string]string{"app": "web-frontend", "env": "staging"},
	})
	fmt.Printf("[%s] → INTENT: %s\n", caller, intentMsg.ID)

	state := opp.StatePending
	fmt.Printf("  State: %s\n", state)

	// Step 2: Worker accepts
	acceptMsg := opp.NewAccept(worker, caller, sess, task)
	state, _ = opp.Transition(state, acceptMsg)
	fmt.Printf("[%s] → ACCEPT\n  State: %s\n", worker, state)

	// Step 3: Worker reports progress
	progMsg := opp.NewProgress(worker, caller, sess, task, "npm_install", 0.3, "Installing dependencies...")
	state, _ = opp.Transition(state, progMsg)
	fmt.Printf("[%s] → PROGRESS: 30%%\n  State: %s\n", worker, state)

	// Step 4: Worker encounters a problem
	problemMsg := opp.NewProblem(worker, caller, sess, task, opp.ProblemPayload{
		Severity:    "error",
		Category:    "port_conflict",
		Description: "Port 3000 is already in use by another process",
		Options: []opp.ProblemOption{
			{Value: "kill", Label: "Kill existing process", Risk: "moderate"},
			{Value: "alt_port", Label: "Use port 3001 instead", Risk: "safe"},
			{Value: "abort", Label: "Abort deploy", Risk: "safe"},
		},
	})
	state, _ = opp.Transition(state, problemMsg)
	fmt.Printf("[%s] → PROBLEM: port conflict\n  State: %s\n", worker, state)

	// Step 5: Caller makes a decision
	decideMsg := opp.NewDecide(caller, worker, sess, task, "", "alt_port", "safer option")
	state, _ = opp.Transition(state, decideMsg)
	fmt.Printf("[%s] → DECIDE: alt_port\n  State: %s\n", caller, state)

	// Step 6: Worker completes
	resultMsg := opp.NewResult(worker, caller, sess, task, "success",
		map[string]string{"url": "https://staging.example.com:3001"}, nil)
	state, _ = opp.Transition(state, resultMsg)
	fmt.Printf("[%s] → RESULT: success\n  State: %s\n", worker, state)

	// Validate + Serialize
	if err := intentMsg.Validate(); err != nil {
		fmt.Println("Validation failed:", err)
		return
	}
	data, _ := intentMsg.Bytes()
	fmt.Printf("\nSerialized: %d bytes\n", len(data))

	parsed, _ := opp.ParseMessage(data)
	intent, _ := parsed.DecodeIntent()
	fmt.Printf("Parsed intent: %s v%s\n", intent.Intent.Name, intent.Intent.Version)
}
