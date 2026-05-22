package planner

import (
	"context"
	"time"

	"yunque-agent/internal/agentcore/skillgrowth"
)

type skillGenerateFuncAdapter struct {
	generate SkillGenerateFunc
}

// NewSkillGrowthPipelineGenerator adapts the existing planner skill generator
// to the canonical skill-growth generation stage. The generated skill remains
// a candidate in the shared pipeline event stream even though the legacy
// generator still registers it immediately.
func NewSkillGrowthPipelineGenerator(generate SkillGenerateFunc) skillgrowth.GenerationStage {
	return &skillGenerateFuncAdapter{generate: generate}
}

func (a *skillGenerateFuncAdapter) GenerateCandidate(ctx context.Context, gap skillgrowth.Gap) (*skillgrowth.Candidate, error) {
	registeredName, err := a.generate(ctx, gap.Description, gap.FailureContext)
	if err != nil {
		return nil, err
	}
	return &skillgrowth.Candidate{
		ID:           registeredName,
		CapabilityID: gap.CapabilityID,
		Name:         registeredName,
		Description:  gap.Description,
		Source:       "planner.skill_generator",
		CreatedAt:    time.Now(),
		Metadata: map[string]string{
			"legacy_registration": "immediate",
		},
	}, nil
}
