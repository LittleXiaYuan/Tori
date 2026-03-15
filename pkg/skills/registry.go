package skills

import "context"

// Skill is an atomic capability unit that the planner can invoke.
type Skill interface {
	// Name returns the unique skill identifier.
	Name() string
	// Description returns a human-readable description for the planner.
	Description() string
	// Parameters returns JSON schema of expected input.
	Parameters() map[string]any
	// Execute runs the skill with given arguments and context.
	Execute(ctx context.Context, args map[string]any, env *Environment) (string, error)
}

// LLMCallFunc calls the LLM with a system prompt and user prompt, returning the response.
type LLMCallFunc func(ctx context.Context, system, user string) (string, error)

// MemorySearchFunc searches memory for relevant context.
type MemorySearchFunc func(ctx context.Context, tenantID, query string, topK int) (string, error)

// Environment provides shared resources to skills.
type Environment struct {
	ClassID      string
	TeacherID    string
	StudentID    string
	TenantID     string
	LLMCall      LLMCallFunc
	MemorySearch MemorySearchFunc
}

// Registry holds all registered skills.
type Registry struct {
	skills  map[string]Skill
	version int // monotonically increasing counter, incremented on Register/Clear
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]Skill)}
}

// Clear removes all skills from the registry.
func (r *Registry) Clear() {
	r.skills = make(map[string]Skill)
	r.version++
}

// Register adds a skill to the registry.
func (r *Registry) Register(s Skill) {
	r.skills[s.Name()] = s
	r.version++
}

// Version returns a monotonically increasing counter that changes whenever skills are added or removed.
func (r *Registry) Version() int {
	return r.version
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	s, ok := r.skills[name]
	return s, ok
}

// All returns all registered skills.
func (r *Registry) All() []Skill {
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// Definitions returns tool definitions for the planner prompt.
func (r *Registry) Definitions() []map[string]any {
	var defs []map[string]any
	for _, s := range r.skills {
		defs = append(defs, map[string]any{
			"name":        s.Name(),
			"description": s.Description(),
			"parameters":  s.Parameters(),
		})
	}
	return defs
}
