// Package cognicore contains cognition modules that are now part of the
// supported mainline runtime rather than speculative experiments.
//
// The first promoted modules are:
//   - trait: durable learned preference dimensions,
//   - recommend: runtime skill/style recommendation,
//   - react: Ledger-backed ReAct / Plan-Execute-Reflect loops,
//   - taskdistill: post-task experience distillation,
//   - eval: task-quality evaluation and distillation trigger decisions.
//   - causal: failure/root-cause reasoning over Ledger events,
//   - curiosity: idle knowledge-gap and failure-review exploration,
//   - world: tracked external state and action-impact prediction.
//   - microagent: scoped prompt-enhancement snippets and registry.
//   - metacog: real-time reasoning anomaly monitor and alert bridge.
//
// Packages that still live under internal/experimental are either true lab
// work or compatibility adapters waiting for a canonical owner.
package cognicore
