package gateway

import (
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

// SkillsRegistry exposes the skill registry to backend packs (e.g. the skills
// pack's native list / dynamic handlers). May be nil if not configured.
func (g *Gateway) SkillsRegistry() *skills.Registry { return g.registry }

// Metrics exposes the metrics tracker to backend packs (e.g. for per-skill usage
// stats in the skills listing). May be nil.
func (g *Gateway) Metrics() *observe.Metrics { return g.metrics }

// ScanSkills triggers a filesystem rescan of data/skills via the skill file
// loader and returns (skills loaded, loader configured). Exposed for the skills
// pack's native /v1/skills/scan handler. Reports configured=false (not 0) when no
// loader is wired, so the pack preserves the original "not configured" error.
func (g *Gateway) ScanSkills() (int, bool) {
	if g.skillFileLoader == nil {
		return 0, false
	}
	return g.skillFileLoader.LoadAll(), true
}

// The full /v1/skills/* surface (listing, scan, dynamic, approve, reject) was
// de-shelled into the skills pack (internal/packs/skills); their handler logic
// now lives there. The pack reads the registry / metrics / file-scanner through
// the accessors above. No gateway bridge remains.
