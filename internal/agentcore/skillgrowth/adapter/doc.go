// Package adapter detects repeated user patterns and proposes generated
// skills.
//
// It now owns only the detect/generate adapter pieces of the canonical
// internal/agentcore/skillgrowth pipeline. New lifecycle, review, promotion, or
// rollback logic should be implemented behind that pipeline instead of growing
// this package into another owner.
package adapter
