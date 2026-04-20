package skillgrow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/pkg/jsonutil"
)

// LLMFunc abstracts an LLM call for skill code generation.
type LLMFunc func(ctx context.Context, system, user string) (string, error)

// SkillTemplate is a generated skill proposal.
type SkillTemplate struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  []SkillParam      `json:"parameters"`
	Code        string            `json:"code"`
	Trigger     string            `json:"trigger"`  // pattern that triggers this skill
	Confidence  float64           `json:"confidence"`
	Source      string            `json:"source"`   // "skillgrow" | "user_request"
	CreatedAt   time.Time         `json:"created_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SkillParam describes a parameter for the generated skill.
type SkillParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// FeedbackKind indicates what the user thinks of a proposal.
type FeedbackKind string

const (
	FeedbackAccept  FeedbackKind = "accept"
	FeedbackReject  FeedbackKind = "reject"
	FeedbackModify  FeedbackKind = "modify"
	FeedbackIgnore  FeedbackKind = "ignore"
)

// Generator creates new skills from detected patterns.
type Generator struct {
	mu          sync.Mutex
	llmCall     LLMFunc
	proposals   map[string]*SkillTemplate // slug → template
	accepted    map[string]*SkillTemplate
	onGenerated func(template SkillTemplate)
}

// NewGenerator creates a skill generator.
func NewGenerator(llmCall LLMFunc) *Generator {
	return &Generator{
		llmCall:   llmCall,
		proposals: make(map[string]*SkillTemplate),
		accepted:  make(map[string]*SkillTemplate),
	}
}

// SetOnGenerated sets the callback when a skill template is generated.
func (g *Generator) SetOnGenerated(fn func(SkillTemplate)) {
	g.onGenerated = fn
}

// GenerateFromPattern takes a detected pattern and generates a skill template.
func (g *Generator) GenerateFromPattern(ctx context.Context, pattern Pattern) (*SkillTemplate, error) {
	if g.llmCall == nil {
		return nil, fmt.Errorf("no LLM configured")
	}

	system := `You are a skill code generator. Given a repeated user pattern, create a reusable automation skill.
Output ONLY valid JSON:
{
  "slug": "snake_case_name",
  "name": "Human Readable Name",
  "description": "What this skill does",
  "parameters": [{"name":"param1","type":"string","description":"desc","required":true}],
  "code": "// Go function body that implements the skill\nfunc(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {\n  // implementation\n  return \"result\", nil\n}",
  "trigger": "pattern description"
}`

	user := fmt.Sprintf("Detected pattern (seen %d times):\nQuery: %s\nSample response: %s\n\nGenerate a reusable skill.",
		pattern.Count, pattern.Query, truncate(pattern.Sample, 500))

	reply, err := g.llmCall(ctx, system, user)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	template := &SkillTemplate{}
	if err := jsonutil.Unmarshal(reply, template); err != nil {
		return nil, fmt.Errorf("parse template: %w (raw: %s)", err, truncate(reply, 200))
	}

	template.Source = "skillgrow"
	template.CreatedAt = time.Now()
	template.Confidence = float64(pattern.Count) / 10.0
	if template.Confidence > 1.0 {
		template.Confidence = 1.0
	}

	g.mu.Lock()
	g.proposals[template.Slug] = template
	g.mu.Unlock()

	slog.Info("skillgrow: generated template", "slug", template.Slug, "pattern_count", pattern.Count)

	if g.onGenerated != nil {
		g.onGenerated(*template)
	}

	return template, nil
}

// Feedback processes user feedback on a proposed skill.
func (g *Generator) Feedback(slug string, kind FeedbackKind, modification string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	template, ok := g.proposals[slug]
	if !ok {
		return fmt.Errorf("no proposal with slug: %s", slug)
	}

	switch kind {
	case FeedbackAccept:
		g.accepted[slug] = template
		delete(g.proposals, slug)
		slog.Info("skillgrow: skill accepted", "slug", slug)

	case FeedbackReject:
		delete(g.proposals, slug)
		slog.Info("skillgrow: skill rejected", "slug", slug)

	case FeedbackModify:
		if modification != "" {
			template.Description = modification
		}
		slog.Info("skillgrow: skill modified", "slug", slug)

	case FeedbackIgnore:
		// keep in proposals but don't act
	}

	return nil
}

// Proposals returns all pending skill proposals.
func (g *Generator) Proposals() []SkillTemplate {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]SkillTemplate, 0, len(g.proposals))
	for _, t := range g.proposals {
		out = append(out, *t)
	}
	return out
}

// AcceptedSkills returns all accepted/generated skills.
func (g *Generator) AcceptedSkills() []SkillTemplate {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]SkillTemplate, 0, len(g.accepted))
	for _, t := range g.accepted {
		out = append(out, *t)
	}
	return out
}

