// Package skillgrowth defines the canonical skill-growth pipeline.
//
// Skill growth used to be split across planner-side missing-capability
// acquisition, experimental pattern detection/generation, and selfheal
// candidate promotion/rollback. This package is the shared boundary for that
// data flow:
//
//	detect → generate → review → promote → observe → rollback
//
// Existing implementations may keep their current packages while adapting to
// this pipeline. New skill creation or retirement logic should enter through
// Pipeline stages instead of adding another owner.
package skillgrowth
