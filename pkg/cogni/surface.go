package cogni

import (
	"yunque-agent/pkg/skills"
)

// SurfaceInput is the pre-filter set of skills plus per-skill metadata the
// evaluator needs to apply a ToolSurface.
type SurfaceInput struct {
	// Skill is the candidate skill.
	Skill skills.Skill
	// Capsule is the owning capsule ID (empty for skills not bound to any
	// capsule, e.g. dynamic skills from data/skills).
	Capsule string
}

// Surface applies a ToolSurface filter against the given candidates.
// Empty/zero-valued surface is the identity filter (returns all candidates).
//
// Filtering order (matches ToolSurface doc):
//  1. FromCapsules narrows by owner.
//  2. Only restricts to an explicit name list.
//  3. Exclude removes named skills.
//  4. Include re-adds named skills (even if previously excluded).
//  5. MaxTools caps the length.
func Surface(candidates []SurfaceInput, s ToolSurface) []skills.Skill {
	if len(candidates) == 0 {
		return nil
	}

	filtered := candidates

	if len(s.FromCapsules) > 0 {
		set := make(map[string]bool, len(s.FromCapsules))
		for _, c := range s.FromCapsules {
			set[c] = true
		}
		filtered = filterInPlace(filtered, func(c SurfaceInput) bool { return set[c.Capsule] })
	}

	if len(s.Only) > 0 {
		set := make(map[string]bool, len(s.Only))
		for _, n := range s.Only {
			set[n] = true
		}
		filtered = filterInPlace(filtered, func(c SurfaceInput) bool { return set[c.Skill.Name()] })
	}

	if len(s.Exclude) > 0 {
		set := make(map[string]bool, len(s.Exclude))
		for _, n := range s.Exclude {
			set[n] = true
		}
		filtered = filterInPlace(filtered, func(c SurfaceInput) bool { return !set[c.Skill.Name()] })
	}

	out := make([]skills.Skill, 0, len(filtered))
	seen := make(map[string]bool, len(filtered))
	for _, c := range filtered {
		name := c.Skill.Name()
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, c.Skill)
	}

	if len(s.Include) > 0 {
		cIdx := make(map[string]skills.Skill, len(candidates))
		for _, c := range candidates {
			cIdx[c.Skill.Name()] = c.Skill
		}
		for _, n := range s.Include {
			if seen[n] {
				continue
			}
			if sk, ok := cIdx[n]; ok {
				out = append(out, sk)
				seen[n] = true
			}
		}
	}

	if s.MaxTools > 0 && len(out) > s.MaxTools {
		out = out[:s.MaxTools]
	}
	return out
}

// MergeSurfaces combines multiple ToolSurface outputs into a deduplicated set.
// Later surfaces add to the union of earlier ones.
func MergeSurfaces(surfaces ...[]skills.Skill) []skills.Skill {
	seen := make(map[string]bool)
	var out []skills.Skill
	for _, s := range surfaces {
		for _, sk := range s {
			if seen[sk.Name()] {
				continue
			}
			seen[sk.Name()] = true
			out = append(out, sk)
		}
	}
	return out
}

func filterInPlace(in []SurfaceInput, pred func(SurfaceInput) bool) []SurfaceInput {
	out := in[:0]
	for _, c := range in {
		if pred(c) {
			out = append(out, c)
		}
	}
	return out
}
