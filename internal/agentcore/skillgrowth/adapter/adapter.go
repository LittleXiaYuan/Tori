package adapter

import (
	"context"
	"fmt"
	"strconv"

	"yunque-agent/internal/agentcore/skillgrowth"
)

// PipelineGenerator adapts Generator to the canonical skill-growth generation
// stage. It keeps generated templates as candidates until a lifecycle/promoter
// stage decides what should be installed.
type PipelineGenerator struct {
	Generator *Generator
}

func NewPipelineGenerator(generator *Generator) *PipelineGenerator {
	return &PipelineGenerator{Generator: generator}
}

func (g *PipelineGenerator) GenerateCandidate(ctx context.Context, gap skillgrowth.Gap) (*skillgrowth.Candidate, error) {
	if g == nil || g.Generator == nil {
		return nil, ErrNoGenerator
	}
	template, err := g.Generator.GenerateFromPattern(ctx, Pattern{
		Query:      gap.Description,
		Count:      patternCount(gap),
		Sample:     gap.FailureContext,
		DetectedAt: gap.DetectedAt,
		Proposed:   true,
	})
	if err != nil {
		return nil, err
	}
	return CandidateFromTemplate(gap.CapabilityID, *template), nil
}

// CandidateFromTemplate maps a generated template to the canonical candidate
// shape shared with review/promote/rollback stages.
func CandidateFromTemplate(capabilityID string, template SkillTemplate) *skillgrowth.Candidate {
	return &skillgrowth.Candidate{
		ID:           template.Slug,
		CapabilityID: capabilityID,
		Name:         template.Name,
		Description:  template.Description,
		Source:       template.Source,
		CreatedAt:    template.CreatedAt,
		Metadata: map[string]string{
			"slug":       template.Slug,
			"trigger":    template.Trigger,
			"confidence": fmt.Sprintf("%.3f", template.Confidence),
		},
	}
}

func patternCount(gap skillgrowth.Gap) int {
	if gap.Evidence == nil {
		return 1
	}
	n, err := strconv.Atoi(gap.Evidence["count"])
	if err != nil || n <= 0 {
		return 1
	}
	return n
}
