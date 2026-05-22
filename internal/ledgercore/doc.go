// Package ledger provides an AI-native state infrastructure for agent runtimes.
//
// Ledger manages task lifecycle, event logs, checkpoints, structured memory,
// and artifact metadata. It is designed as an embeddable library with zero
// external dependencies in its default configuration (SQLite backend).
//
// Usage:
//
//	backend, err := sqlite.New("ledger.db")
//	if err != nil { log.Fatal(err) }
//	ldg, err := ledger.Open(backend)
//	if err != nil { log.Fatal(err) }
//	defer ldg.Close()
package ledger
