// Package capsule defines the Capsule — a first-class capability unit
// that unifies installation, runtime, capability, and AI-cognition boundaries.
//
// A Capsule is:
//   - An **install unit**: declarative manifest with version, deps, permissions, resources.
//   - A **runtime unit**: can run in-process, as a sidecar, or in a container.
//   - A **capability unit**: exposes Skills / UI / HTTP / Providers / Channels / Events
//     through a single declarative Exports structure.
//   - An **AI-cognition unit**: pairs with a Cogni declaration for scenario-based
//     activation, tool-surface filtering, and memory policy.
//
// Capsule supersedes the legacy plugin.Plugin abstraction, while remaining
// backward-compatible via adapter helpers in pkg/plugin.
//
// Status (2026-04): the Capsule + Cogni model is fully implemented in this
// package and pkg/cogni, with tests, but is **not yet wired into the
// running agent** — `cmd/agent` and `internal/*` continue to use the
// pkg/plugin / pkg/skills path. Use this package only for new
// experimental capsules; production wiring is tracked as the next
// architectural milestone after the dual plugin/capsule system is
// reconciled. Until then, do not delete the package: it represents a
// load-bearing design contract referenced by the architecture docs.
package capsule

import (
	"context"
	"fmt"
)

// Capsule is a first-class capability unit.
//
// Every Capsule has a Manifest (declarative metadata), a Runtime (execution
// environment), and Exports (the capabilities it contributes to the host).
type Capsule interface {
	// Manifest returns the declarative descriptor of this capsule.
	// MUST return the same pointer across calls — the registry stores it.
	Manifest() *Manifest

	// Exports returns everything the capsule contributes to the host when
	// activated: skills, UI tabs, HTTP routes, providers, channels, etc.
	// Returned value MAY change between Activate/Suspend cycles.
	Exports() *Exports

	// Runtime returns the runtime driver for this capsule.
	// Most built-in capsules return an InProcessRuntime; heavy capsules
	// (e.g. digital-human live) may return a SidecarRuntime or ContainerRuntime.
	Runtime() Runtime
}

// CapsuleID is the unique identifier of a Capsule (matches Manifest.Name).
type CapsuleID = string

// Descriptor is a serializable summary of a capsule — used for API responses
// and UI rendering. It intentionally excludes live handlers.
type Descriptor struct {
	ID          CapsuleID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name,omitempty"`
	Version     string    `json:"version"`
	Author      string    `json:"author,omitempty"`
	Description string    `json:"description,omitempty"`
	Category    string    `json:"category,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	Source      string    `json:"source"` // "builtin" | "installed" | "script" | "sidecar"

	State   State    `json:"state"`
	Enabled bool     `json:"enabled"`
	Slot    string   `json:"slot,omitempty"`

	SkillCount   int      `json:"skill_count"`
	UITabCount   int      `json:"ui_tab_count"`
	RouteCount   int      `json:"route_count"`
	RuntimeKind  RuntimeKind `json:"runtime_kind"`
	HasCogni     bool     `json:"has_cogni"`

	Permissions []string `json:"permissions,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// DescribeCapsule produces a Descriptor from a Capsule and its current state.
func DescribeCapsule(c Capsule, state State, enabled bool, source string) Descriptor {
	m := c.Manifest()
	exp := c.Exports()
	rtKind := RuntimeInProcess
	if rt := c.Runtime(); rt != nil {
		rtKind = rt.Kind()
	}

	var perms []string
	for _, p := range m.Permissions {
		perms = append(perms, p.Name)
	}
	var deps []string
	for _, d := range m.Dependencies {
		deps = append(deps, d.Name)
	}

	skillCount, tabCount, routeCount := 0, 0, 0
	if exp != nil {
		skillCount = len(exp.Skills)
		tabCount = len(exp.UITabs)
		routeCount = len(exp.HTTPRoutes)
	}

	return Descriptor{
		ID:          m.Name,
		Name:        m.Name,
		DisplayName: m.DisplayName,
		Version:     m.Version,
		Author:      m.Author,
		Description: m.Description,
		Category:    m.Category,
		Icon:        m.Icon,
		Source:      source,
		State:       state,
		Enabled:     enabled,
		Slot:        m.Slot,
		SkillCount:  skillCount,
		UITabCount:  tabCount,
		RouteCount:  routeCount,
		RuntimeKind: rtKind,
		HasCogni:    m.Cogni != nil,
		Permissions: perms,
		Dependencies: deps,
	}
}

// Env is the shared environment passed to Runtime.Start / Exports handlers.
// Capsules use this to call back into host services (LLM, memory, data dir)
// without depending on the concrete types.
type Env struct {
	// DataDir is the capsule-private data directory (already created).
	DataDir string

	// APIToken is a capsule-scoped token for calling the host's internal API.
	// Empty for unauthenticated capsules.
	APIToken string

	// LLMCall invokes the host LLM. Capsules SHOULD NOT create their own LLM
	// clients — use this so the host can meter cost, audit, and rotate keys.
	LLMCall func(ctx context.Context, system, user string) (string, error)

	// MemorySearch queries the host memory subsystem.
	// Returns a flattened text suitable for prompt injection.
	MemorySearch func(ctx context.Context, tenantID, query string, topK int) (string, error)

	// TenantID scopes the activation (multi-tenant deployments).
	TenantID string

	// Logger produces structured logs. Capsules SHOULD use it instead of stdout
	// so the host can route logs to its sink.
	Logger Logger
}

// Logger is the minimal logging interface capsules receive via Env.
type Logger interface {
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
}

// Validate performs early sanity checks on a Capsule implementation.
// It is called by Registry.Register — individual capsule authors should not
// need to call it directly.
func Validate(c Capsule) error {
	if c == nil {
		return fmt.Errorf("capsule: nil implementation")
	}
	m := c.Manifest()
	if m == nil {
		return fmt.Errorf("capsule: Manifest() returned nil")
	}
	if err := m.Validate(); err != nil {
		return fmt.Errorf("capsule %q: %w", m.Name, err)
	}
	if c.Runtime() == nil {
		return fmt.Errorf("capsule %q: Runtime() returned nil", m.Name)
	}
	return nil
}
