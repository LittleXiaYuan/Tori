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

// SurfaceMCPTools applies ToolSurface name-based rules to MCP tools for a
// Cogni. FromCapsules is ignored — MCP tools are not bound to skill capsules.
// A zero-valued surface is identity and returns all candidates (already narrowed
// by mcp.tool_filter at connect time).
func SurfaceMCPTools(candidates []MCPToolInfo, s ToolSurface) []MCPToolInfo {
	if len(candidates) == 0 {
		return nil
	}
	if isIdentitySurface(s) {
		return candidates
	}
	idx := make(map[string]MCPToolInfo, len(candidates))
	names := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, t := range candidates {
		if seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		idx[t.Name] = t
		names = append(names, t.Name)
	}
	filtered := surfaceToolNames(names, s)
	out := make([]MCPToolInfo, 0, len(filtered))
	for _, n := range filtered {
		if t, ok := idx[n]; ok {
			out = append(out, t)
		}
	}
	return out
}

// MergeMCPTools unions MCP tool lists from multiple activated cognis, deduping
// by tool name (first cogni wins on collision).
func MergeMCPTools(lists ...[]MCPToolInfo) []MCPToolInfo {
	seen := make(map[string]bool)
	var out []MCPToolInfo
	for _, list := range lists {
		for _, t := range list {
			if seen[t.Name] {
				continue
			}
			seen[t.Name] = true
			out = append(out, t)
		}
	}
	return out
}

// surfaceToolNames applies the name-based ToolSurface rules shared by skills and
// MCP tools. FromCapsules is intentionally excluded — callers apply capsule
// narrowing before invoking this helper when needed.
func surfaceToolNames(names []string, s ToolSurface) []string {
	if len(names) == 0 {
		return nil
	}
	filtered := names

	if len(s.Only) > 0 {
		set := make(map[string]bool, len(s.Only))
		for _, n := range s.Only {
			set[n] = true
		}
		filtered = filterStringsInPlace(filtered, func(n string) bool { return set[n] })
	}

	if len(s.Exclude) > 0 {
		set := make(map[string]bool, len(s.Exclude))
		for _, n := range s.Exclude {
			set[n] = true
		}
		filtered = filterStringsInPlace(filtered, func(n string) bool { return !set[n] })
	}

	candidateSet := make(map[string]bool, len(names))
	for _, n := range names {
		candidateSet[n] = true
	}

	out := append([]string(nil), filtered...)
	seen := make(map[string]bool, len(out))
	for _, n := range out {
		seen[n] = true
	}

	if len(s.Include) > 0 {
		for _, n := range s.Include {
			if seen[n] {
				continue
			}
			if candidateSet[n] {
				out = append(out, n)
				seen[n] = true
			}
		}
	}

	if s.MaxTools > 0 && len(out) > s.MaxTools {
		out = out[:s.MaxTools]
	}
	return out
}

func filterStringsInPlace(in []string, pred func(string) bool) []string {
	out := in[:0]
	for _, s := range in {
		if pred(s) {
			out = append(out, s)
		}
	}
	return out
}

// AllowsName reports whether a tool/skill name survives this surface's
// name-level rules (Only / Include / Exclude). FromCapsules and MaxTools are
// intentionally ignored (they need the full candidate set / ranking), so this is
// a fast membership check used for experience attribution — NOT the
// authoritative filter (that is Surface()/SurfaceMCPTools()).
func (s ToolSurface) AllowsName(name string) bool {
	allowed := true
	if len(s.Only) > 0 {
		allowed = surfaceNameInList(s.Only, name)
	}
	if surfaceNameInList(s.Exclude, name) {
		allowed = false
	}
	if surfaceNameInList(s.Include, name) {
		allowed = true
	}
	return allowed
}

func surfaceNameInList(list []string, name string) bool {
	for _, x := range list {
		if x == name {
			return true
		}
	}
	return false
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
