package planner

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// SkillGrowthConfig controls autonomous skill acquisition behavior.
type SkillGrowthConfig struct {
	Enabled        bool          `json:"enabled"`
	AutoInstall    bool          `json:"auto_install"`    // install without user approval
	SearchTimeout  time.Duration `json:"search_timeout"`
	MaxAutoInstall int           `json:"max_auto_install"` // per-session limit
}

func DefaultSkillGrowthConfig() SkillGrowthConfig {
	return SkillGrowthConfig{
		Enabled:        true,
		AutoInstall:    false,
		SearchTimeout:  10 * time.Second,
		MaxAutoInstall: 3,
	}
}

// SkillSearchFunc searches the skill marketplace for a capability.
// Returns skill slug and description, or empty if not found.
type SkillSearchFunc func(ctx context.Context, query string) (slug, description string, found bool)

// SkillInstallFunc installs a skill by slug and returns the skill name usable in the registry.
type SkillInstallFunc func(ctx context.Context, slug string) (registeredName string, err error)

// SkillGenerateFunc generates a new skill from a capability description using LLM + authoritative sources.
// Returns the registered skill name, or error if generation fails.
type SkillGenerateFunc func(ctx context.Context, capabilityDesc string, failureContext string) (registeredName string, err error)

// SkillGrowth handles autonomous skill acquisition when the planner encounters missing capabilities.
type SkillGrowth struct {
	config   SkillGrowthConfig
	search   SkillSearchFunc
	install  SkillInstallFunc
	generate SkillGenerateFunc

	sessionInstalls int
}

func NewSkillGrowth(cfg SkillGrowthConfig) *SkillGrowth {
	return &SkillGrowth{config: cfg}
}

func (sg *SkillGrowth) SetSearch(fn SkillSearchFunc)     { sg.search = fn }
func (sg *SkillGrowth) SetInstall(fn SkillInstallFunc)   { sg.install = fn }
func (sg *SkillGrowth) SetGenerate(fn SkillGenerateFunc) { sg.generate = fn }

// TryAcquire attempts to find and install a missing skill.
// Returns the registered skill name if successful, or empty string + reason if not.
func (sg *SkillGrowth) TryAcquire(ctx context.Context, skillName string, failureContext string) (string, string) {
	if sg == nil || !sg.config.Enabled {
		return "", "skill growth disabled"
	}

	if sg.sessionInstalls >= sg.config.MaxAutoInstall {
		return "", fmt.Sprintf("session auto-install limit reached (%d)", sg.config.MaxAutoInstall)
	}

	searchCtx, cancel := context.WithTimeout(ctx, sg.config.SearchTimeout)
	defer cancel()

	// Phase 1: Search existing skill marketplace
	if sg.search != nil {
		slug, desc, found := sg.search(searchCtx, skillName)
		if found {
			slog.Info("skill_growth: found matching skill in marketplace", "query", skillName, "slug", slug, "desc", desc)

			if !sg.config.AutoInstall {
				return "", fmt.Sprintf("found skill '%s' (%s) but auto-install is disabled — user approval needed", slug, desc)
			}

			if sg.install != nil {
				name, err := sg.install(ctx, slug)
				if err != nil {
					slog.Warn("skill_growth: install failed", "slug", slug, "err", err)
					return "", fmt.Sprintf("found '%s' but install failed: %v", slug, err)
				}
				sg.sessionInstalls++
				slog.Info("skill_growth: auto-installed skill", "slug", slug, "registered_as", name)
				return name, ""
			}
		}
	}

	// Phase 2: Generate new skill from authoritative sources
	if sg.generate != nil {
		slog.Info("skill_growth: attempting skill generation", "capability", skillName)
		name, err := sg.generate(ctx, skillName, failureContext)
		if err != nil {
			slog.Warn("skill_growth: generation failed", "capability", skillName, "err", err)
			return "", fmt.Sprintf("skill generation failed: %v", err)
		}
		sg.sessionInstalls++
		slog.Info("skill_growth: generated and registered new skill", "capability", skillName, "registered_as", name)
		return name, ""
	}

	return "", "no skill providers configured"
}
