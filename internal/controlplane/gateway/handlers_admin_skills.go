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

// The skill listing (/v1/skills) and the dynamic skill surface
// (/v1/skills/dynamic, /approve, /reject) were de-shelled into the skills pack
// (internal/packs/skills); their handler logic now lives there. Only
// /v1/skills/scan remains on the gateway bridge (HandleSkillsPack).
