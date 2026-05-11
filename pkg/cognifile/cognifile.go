// Package cognifile defines the Cognifile — the "Dockerfile for AI Agents".
//
// A Cognifile is a declarative YAML/JSON specification that fully describes
// a cognitive agent: its persona, skills, activation rules, model
// requirements, memory policy, MCP connections, and runtime constraints.
// Like a Dockerfile produces a container image, a Cognifile produces a
// runnable Cogni instance that can be pulled, shared, and versioned.
//
// Design principles:
//   - Human-readable YAML as the primary authoring format
//   - Maps cleanly to the existing cogni.Declaration for zero-friction integration
//   - Self-contained: everything needed to run the agent is in one file
//   - Versionable: can be committed to git and diffed meaningfully
//   - Composable: supports FROM-based inheritance from base templates
package cognifile

import (
	"fmt"
	"strings"
	"time"

	"yunque-agent/pkg/cogni"
)

// SchemaVersion is the current Cognifile schema version.
const SchemaVersion = "cognifile/v1"

// Cognifile is the top-level declarative specification for a cognitive agent.
//
// A minimal Cognifile needs only `name` and `persona.role`; everything else
// has sensible defaults. A full Cognifile can specify model requirements,
// activation rules, tool surface, MCP servers, workflows, and self-tests.
type Cognifile struct {
	// Schema identifies the format version (e.g. "cognifile/v1").
	// Readers MUST check this before interpreting the rest.
	Schema string `json:"schema" yaml:"schema"`

	// From references a base Cognifile to inherit from. The current
	// Cognifile's fields override the base. Empty means no inheritance.
	From string `json:"from,omitempty" yaml:"from,omitempty"`

	// Name is the unique identifier for this Cognifile (analogous to a
	// container image name). Convention: lowercase, hyphens, no spaces.
	Name string `json:"name" yaml:"name"`

	// DisplayName is the user-facing label (may contain CJK / emoji).
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`

	// Version follows semver (e.g. "1.0.0").
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// Description is a short summary of what the agent does.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Author is the creator's name or organization.
	Author string `json:"author,omitempty" yaml:"author,omitempty"`

	// Tags are free-form labels for search and categorization.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	// Icon is a lucide-react icon name or emoji for the UI.
	Icon string `json:"icon,omitempty" yaml:"icon,omitempty"`

	// Persona defines the agent's identity and behavioral characteristics.
	Persona Persona `json:"persona" yaml:"persona"`

	// Model declares the LLM requirements.
	Model ModelSpec `json:"model,omitempty" yaml:"model,omitempty"`

	// Activation declares when this agent engages (keyword/regex/channel rules).
	Activation cogni.ActivationRules `json:"activation,omitempty" yaml:"activation,omitempty"`

	// Skills declares what tools/capabilities the agent has access to.
	Skills SkillSurface `json:"skills,omitempty" yaml:"skills,omitempty"`

	// Context declares supplemental text injected into the system prompt.
	Context cogni.ContextInjection `json:"context,omitempty" yaml:"context,omitempty"`

	// Memory declares how facts are stored and retrieved.
	Memory cogni.MemoryPolicy `json:"memory,omitempty" yaml:"memory,omitempty"`

	// MCP declares per-agent MCP server connections.
	MCP cogni.MCPConfig `json:"mcp,omitempty" yaml:"mcp,omitempty"`

	// Workflows declares multi-step automation workflows.
	Workflows []cogni.WorkflowDef `json:"workflows,omitempty" yaml:"workflows,omitempty"`

	// Economics declares resource constraints (budget, rate limits).
	Economics cogni.EconomicsConfig `json:"economics,omitempty" yaml:"economics,omitempty"`

	// Checks are self-tests that verify activation rules behave as intended.
	Checks []cogni.ActivationCheck `json:"checks,omitempty" yaml:"checks,omitempty"`

	// Runtime declares execution environment preferences.
	Runtime RuntimeSpec `json:"runtime,omitempty" yaml:"runtime,omitempty"`

	// Priority for tie-breaking when multiple agents activate (lower = higher priority).
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`

	// Exclusive group: only one agent per group may activate per turn.
	Exclusive string `json:"exclusive,omitempty" yaml:"exclusive,omitempty"`

	// Metadata stores arbitrary key-value pairs for extensibility.
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Persona defines the agent's identity, role, and behavioral guardrails.
type Persona struct {
	// Role is the one-line identity statement (e.g. "资深法律顾问").
	Role string `json:"role" yaml:"role"`

	// SystemPrompt is the full system prompt prepended to every conversation.
	// If empty, a default prompt is generated from Role + Traits + Constraints.
	SystemPrompt string `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`

	// Traits are adjectives/behaviors the agent embodies (e.g. "严谨", "耐心").
	Traits []string `json:"traits,omitempty" yaml:"traits,omitempty"`

	// Constraints are behavioral guardrails (e.g. "不提供具体投资建议").
	Constraints []string `json:"constraints,omitempty" yaml:"constraints,omitempty"`

	// Greeting is the agent's opening message when a conversation starts.
	Greeting string `json:"greeting,omitempty" yaml:"greeting,omitempty"`

	// Language is the primary response language (e.g. "zh-CN", "en").
	Language string `json:"language,omitempty" yaml:"language,omitempty"`

	// Tone controls formality: "formal", "casual", "professional", "friendly".
	Tone string `json:"tone,omitempty" yaml:"tone,omitempty"`
}

// ModelSpec declares the LLM requirements for this agent.
type ModelSpec struct {
	// Provider is the LLM provider preference (e.g. "openai", "deepseek", "qwen").
	// Empty means use the host's default pool.
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`

	// Tier selects the model tier from the host's pool: "fast", "smart", "expert".
	Tier string `json:"tier,omitempty" yaml:"tier,omitempty"`

	// Name is a specific model name override (e.g. "gpt-4o", "deepseek-chat").
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Temperature controls randomness (0.0-2.0). 0 means use default.
	Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`

	// MaxTokens caps the response length. 0 means use default.
	MaxTokens int `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`

	// TopP controls nucleus sampling. 0 means use default.
	TopP float64 `json:"top_p,omitempty" yaml:"top_p,omitempty"`
}

// SkillSurface declares tool/skill access, extending cogni.ToolSurface with
// inline skill definitions for self-contained Cognifiles.
type SkillSurface struct {
	cogni.ToolSurface `json:",inline" yaml:",inline"`

	// Builtin lists built-in skill names to enable (e.g. "web_search", "code_run").
	Builtin []string `json:"builtin,omitempty" yaml:"builtin,omitempty"`
}

// RuntimeSpec declares execution environment preferences.
type RuntimeSpec struct {
	// Sandbox enables sandboxed execution for tool calls.
	Sandbox bool `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`

	// MaxConcurrency caps parallel tool invocations.
	MaxConcurrency int `json:"max_concurrency,omitempty" yaml:"max_concurrency,omitempty"`

	// TimeoutSec is the per-turn timeout.
	TimeoutSec int `json:"timeout_sec,omitempty" yaml:"timeout_sec,omitempty"`

	// TrustLevel sets the initial trust score (0-100).
	TrustLevel int `json:"trust_level,omitempty" yaml:"trust_level,omitempty"`
}

// InstalledCognifile is a Cognifile that has been pulled/installed locally.
type InstalledCognifile struct {
	Cognifile   *Cognifile `json:"cognifile"`
	InstalledAt time.Time  `json:"installed_at"`
	Source      string     `json:"source"` // "local", "hub:<url>", "file:<path>"
	FilePath    string     `json:"file_path,omitempty"`
}

// Validate checks that the Cognifile has all required fields and consistent values.
func (c *Cognifile) Validate() error {
	if c == nil {
		return fmt.Errorf("cognifile: nil")
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("cognifile: name is required")
	}
	if strings.ContainsAny(c.Name, " \t\n") {
		return fmt.Errorf("cognifile: name must not contain whitespace (got %q)", c.Name)
	}
	if strings.TrimSpace(c.Persona.Role) == "" {
		return fmt.Errorf("cognifile: persona.role is required")
	}
	if c.Model.Temperature < 0 || c.Model.Temperature > 2 {
		return fmt.Errorf("cognifile: model.temperature must be 0-2 (got %v)", c.Model.Temperature)
	}
	if c.Runtime.TrustLevel < 0 || c.Runtime.TrustLevel > 100 {
		return fmt.Errorf("cognifile: runtime.trust_level must be 0-100 (got %d)", c.Runtime.TrustLevel)
	}
	return nil
}

// ToDeclaration converts a Cognifile into a cogni.Declaration that can be
// registered with the existing cogni.Registry. This is the bridge between
// the Cognifile world and the runtime world.
func (c *Cognifile) ToDeclaration() *cogni.Declaration {
	d := &cogni.Declaration{
		ID:          c.Name,
		DisplayName: c.DisplayName,
		Description: c.Description,
		Activation:  c.Activation,
		Surface:     c.Skills.ToolSurface,
		Context:     c.buildContext(),
		Memory:      c.Memory,
		MCP:         c.MCP,
		Workflows:   c.Workflows,
		Economics:    c.Economics,
		Checks:      c.Checks,
		Priority:    c.Priority,
		Exclusive:   c.Exclusive,
	}

	if d.DisplayName == "" {
		d.DisplayName = c.Name
	}

	return d
}

// buildContext merges the Persona into the ContextInjection. If the Cognifile
// has a full SystemPrompt, it becomes the static context. Otherwise, a prompt
// is synthesized from Role + Traits + Constraints.
func (c *Cognifile) buildContext() cogni.ContextInjection {
	ctx := c.Context

	if c.Persona.SystemPrompt != "" {
		if ctx.Static == "" {
			ctx.Static = c.Persona.SystemPrompt
		} else {
			ctx.Static = c.Persona.SystemPrompt + "\n\n" + ctx.Static
		}
		return ctx
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("## 角色\n你是%s。", c.Persona.Role))

	if len(c.Persona.Traits) > 0 {
		parts = append(parts, fmt.Sprintf("## 特质\n- %s", strings.Join(c.Persona.Traits, "\n- ")))
	}

	if len(c.Persona.Constraints) > 0 {
		parts = append(parts, fmt.Sprintf("## 约束\n- %s", strings.Join(c.Persona.Constraints, "\n- ")))
	}

	if c.Persona.Language != "" {
		parts = append(parts, fmt.Sprintf("## 语言\n使用%s回复。", c.Persona.Language))
	}

	if c.Persona.Tone != "" {
		parts = append(parts, fmt.Sprintf("## 语气\n保持%s的语气。", c.Persona.Tone))
	}

	synthesized := strings.Join(parts, "\n\n")
	if ctx.Static != "" {
		ctx.Static = synthesized + "\n\n" + ctx.Static
	} else {
		ctx.Static = synthesized
	}
	return ctx
}
