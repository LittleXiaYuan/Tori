package cognifile

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"yunque-agent/pkg/cogni"
)

// Runner converts Cognifiles into live Cogni declarations and registers
// them with a cogni.Registry. It is the bridge between the declarative
// Cognifile world and the runtime cogni engine.
type Runner struct {
	registry      *cogni.Registry
	localRegistry *LocalRegistry
}

// NewRunner creates a Runner that targets the given cogni.Registry.
func NewRunner(registry *cogni.Registry, localRegistry *LocalRegistry) *Runner {
	return &Runner{
		registry:      registry,
		localRegistry: localRegistry,
	}
}

// RunResult captures the outcome of activating a Cognifile.
type RunResult struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"` // "activated", "already_active", "error"
	Error       string `json:"error,omitempty"`
}

// Run converts a Cognifile to a Declaration and registers it. If the
// Cognifile references a base (FROM), the base is resolved first and
// fields are merged.
func (r *Runner) Run(cf *Cognifile) (*RunResult, error) {
	if err := cf.Validate(); err != nil {
		return nil, fmt.Errorf("cognifile: validate: %w", err)
	}

	if cf.From != "" {
		base, err := r.resolveFrom(cf.From)
		if err != nil {
			return nil, fmt.Errorf("cognifile: resolve FROM %q: %w", cf.From, err)
		}
		cf = merge(base, cf)
	}

	decl := cf.ToDeclaration()

	if existing, ok := r.registry.Get(decl.ID); ok {
		if existing.DisplayName == decl.DisplayName {
			return &RunResult{
				Name:        cf.Name,
				DisplayName: cf.DisplayName,
				Status:      "already_active",
			}, nil
		}
	}

	if err := r.registry.Add(decl, "cognifile:"+cf.Name); err != nil {
		return &RunResult{
			Name:   cf.Name,
			Status: "error",
			Error:  err.Error(),
		}, err
	}

	slog.Info("cognifile: activated",
		"name", cf.Name,
		"display_name", cf.DisplayName,
		"version", cf.Version)

	return &RunResult{
		Name:        cf.Name,
		DisplayName: cf.DisplayName,
		Status:      "activated",
	}, nil
}

// RunByName looks up an installed Cognifile by name and runs it.
func (r *Runner) RunByName(name string) (*RunResult, error) {
	entry, ok := r.localRegistry.Get(name)
	if !ok {
		return nil, fmt.Errorf("cognifile: %q not installed (use 'yunque pull' first)", name)
	}
	return r.Run(entry.Cognifile)
}

// Stop deactivates a running Cognifile by removing its declaration.
func (r *Runner) Stop(name string) bool {
	return r.registry.Remove(name)
}

// resolveFrom looks up a base Cognifile by name from the local registry.
func (r *Runner) resolveFrom(from string) (*Cognifile, error) {
	entry, ok := r.localRegistry.Get(from)
	if !ok {
		return nil, fmt.Errorf("base %q not installed", from)
	}
	return entry.Cognifile, nil
}

// merge applies the overlay's non-zero fields on top of the base.
// The overlay's Name, Schema, From, and Version always win.
func merge(base, overlay *Cognifile) *Cognifile {
	result := *base

	result.Name = overlay.Name
	result.Schema = overlay.Schema
	result.From = ""
	if overlay.Version != "" {
		result.Version = overlay.Version
	}
	if overlay.DisplayName != "" {
		result.DisplayName = overlay.DisplayName
	}
	if overlay.Description != "" {
		result.Description = overlay.Description
	}
	if overlay.Author != "" {
		result.Author = overlay.Author
	}
	if overlay.Icon != "" {
		result.Icon = overlay.Icon
	}
	if len(overlay.Tags) > 0 {
		result.Tags = overlay.Tags
	}

	if overlay.Persona.Role != "" {
		result.Persona.Role = overlay.Persona.Role
	}
	if overlay.Persona.SystemPrompt != "" {
		result.Persona.SystemPrompt = overlay.Persona.SystemPrompt
	}
	if len(overlay.Persona.Traits) > 0 {
		result.Persona.Traits = append(slices.Clone(base.Persona.Traits), overlay.Persona.Traits...)
	}
	if len(overlay.Persona.Constraints) > 0 {
		result.Persona.Constraints = append(slices.Clone(base.Persona.Constraints), overlay.Persona.Constraints...)
	}
	if overlay.Persona.Greeting != "" {
		result.Persona.Greeting = overlay.Persona.Greeting
	}
	if overlay.Persona.Language != "" {
		result.Persona.Language = overlay.Persona.Language
	}
	if overlay.Persona.Tone != "" {
		result.Persona.Tone = overlay.Persona.Tone
	}

	if overlay.Model.Provider != "" || overlay.Model.Tier != "" || overlay.Model.Name != "" {
		result.Model = overlay.Model
	}

	if overlay.Activation.AlwaysOn || len(overlay.Activation.Keywords) > 0 ||
		len(overlay.Activation.Regex) > 0 || len(overlay.Activation.Channels) > 0 {
		result.Activation = overlay.Activation
	}

	if len(overlay.Skills.Only) > 0 || len(overlay.Skills.Include) > 0 ||
		len(overlay.Skills.Builtin) > 0 {
		result.Skills = overlay.Skills
	}

	if overlay.Context.Static != "" || overlay.Context.MemoryQuery != "" ||
		overlay.Context.Template != "" {
		result.Context = overlay.Context
	}

	if overlay.Memory.DropAll || len(overlay.Memory.DropKeys) > 0 ||
		overlay.Memory.Namespace != "" {
		result.Memory = overlay.Memory
	}

	if len(overlay.MCP.Servers) > 0 {
		result.MCP = overlay.MCP
	}

	if len(overlay.Workflows) > 0 {
		result.Workflows = overlay.Workflows
	}

	if len(overlay.Checks) > 0 {
		result.Checks = overlay.Checks
	}

	if overlay.Priority != 0 {
		result.Priority = overlay.Priority
	}
	if overlay.Exclusive != "" {
		result.Exclusive = overlay.Exclusive
	}

	if len(overlay.Metadata) > 0 {
		if result.Metadata == nil {
			result.Metadata = make(map[string]string)
		}
		for k, v := range overlay.Metadata {
			result.Metadata[k] = v
		}
	}

	return &result
}

// FormatCognifileInfo returns a human-readable summary of a Cognifile.
func FormatCognifileInfo(cf *Cognifile) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("名称: %s", cf.Name))
	if cf.DisplayName != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", cf.DisplayName))
	}
	sb.WriteString("\n")

	if cf.Version != "" {
		sb.WriteString(fmt.Sprintf("版本: %s\n", cf.Version))
	}
	if cf.Description != "" {
		sb.WriteString(fmt.Sprintf("描述: %s\n", cf.Description))
	}
	if cf.Author != "" {
		sb.WriteString(fmt.Sprintf("作者: %s\n", cf.Author))
	}
	sb.WriteString(fmt.Sprintf("角色: %s\n", cf.Persona.Role))
	if cf.Model.Tier != "" {
		sb.WriteString(fmt.Sprintf("模型: %s", cf.Model.Tier))
		if cf.Model.Name != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", cf.Model.Name))
		}
		sb.WriteString("\n")
	}
	if len(cf.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("标签: %s\n", strings.Join(cf.Tags, ", ")))
	}
	return sb.String()
}
