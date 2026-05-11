// Package belief exposes the durable belief kernel used to orient an agent
// across sessions without coupling callers to the internal application tree.
//
// The package is intentionally small and dependency-light: it owns belief
// nodes, graph relations, bounded updates, decay, and audit entries, while LLM
// prompting, Cogni loading, and runtime persistence stay in higher layers.
package belief
