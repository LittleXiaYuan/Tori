package capsule

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"yunque-agent/pkg/manifest"
)

// Manifest is the declarative descriptor of a Capsule.
//
// It extends the legacy pkg/manifest.Manifest (which covers name, version,
// permissions, skill declarations, signature) with Capsule-specific metadata:
// display info, categorization, dependencies, resource limits, runtime spec,
// exports declaration, exclusive slots, and an optional Cogni reference.
//
// A Manifest MAY be loaded from JSON/YAML (see LoadManifest) or built
// programmatically by built-in capsules.
type Manifest struct {
	manifest.Manifest // embed basic fields

	// DisplayName is the user-facing name (may contain CJK / emoji).
	// Falls back to Name if empty.
	DisplayName string `json:"display_name,omitempty"`

	// Icon is a lucide-react icon name or a data URI used in the UI.
	Icon string `json:"icon,omitempty"`

	// Category groups capsules in the marketplace / management UI.
	// Well-known values: "messaging", "productivity", "research", "media",
	// "integration", "infrastructure", "experimental".
	Category string `json:"category,omitempty"`

	// Dependencies is the list of other capsules required by this one.
	// The Registry resolves dependencies before activation.
	Dependencies []Dependency `json:"dependencies,omitempty"`

	// Resources declares optional resource limits enforced by the runtime.
	Resources *Resources `json:"resources,omitempty"`

	// Runtime describes how this capsule runs (in-process / sidecar / container).
	// If nil, the runtime defaults to in-process.
	Runtime *RuntimeSpec `json:"runtime,omitempty"`

	// Slot declares an exclusive slot (only one capsule may occupy a slot).
	// Example: "channel:whatsapp", "memory:long-term".
	Slot string `json:"slot,omitempty"`

	// Cogni is an optional reference to a Cogni declaration — the AI-cognition
	// shell paired with this capsule. If nil, the capsule is not AI-exposed by
	// default (the host will still surface its skills through the planner, but
	// without activation rules / tool filtering / memory policy).
	Cogni *CogniRef `json:"cogni,omitempty"`

	// Source identifies how the capsule was obtained:
	//   "builtin"   — compiled into the host binary
	//   "installed" — installed from the marketplace into the user's data dir
	//   "script"    — user-authored script in data/plugins
	//   "sidecar"   — external binary managed as a subprocess
	// The Registry fills this in if empty.
	Source string `json:"source,omitempty"`

	// Homepage / Repository / Documentation are marketplace metadata.
	Homepage      string `json:"homepage,omitempty"`
	Repository    string `json:"repository,omitempty"`
	Documentation string `json:"documentation,omitempty"`

	// Tags are free-form search labels.
	Tags []string `json:"tags,omitempty"`
}

// Dependency declares a required or optional dependency on another Capsule.
type Dependency struct {
	// Name is the Capsule ID being depended on.
	Name string `json:"name"`

	// Version is a semver range (e.g. ">=1.0.0 <2.0.0"). Empty means any.
	Version string `json:"version,omitempty"`

	// Optional dependencies do not block activation when missing; the
	// capsule is expected to handle absence gracefully.
	Optional bool `json:"optional,omitempty"`

	// Reason is a human-readable explanation surfaced in the UI.
	Reason string `json:"reason,omitempty"`
}

// Resources declares resource limits that sidecar / container runtimes enforce.
// For in-process runtimes these are advisory (surfaced in the UI for users
// to inspect resource cost before enabling).
type Resources struct {
	MaxMemoryMB int `json:"max_memory_mb,omitempty"`
	MaxCPUPct   int `json:"max_cpu_pct,omitempty"`
	DiskMB      int `json:"disk_mb,omitempty"`
	// Network declares outbound traffic bytes/sec cap (0 = unlimited).
	NetworkKBps int `json:"network_kbps,omitempty"`
}

// RuntimeSpec describes how a Capsule executes. Only one of the runtime-kind
// specific fields applies, chosen by Kind.
type RuntimeSpec struct {
	// Kind selects the runtime driver.
	Kind RuntimeKind `json:"kind"`

	// Entry is the executable path (for sidecar) or entry command
	// (for container). Relative to the capsule install directory.
	Entry string `json:"entry,omitempty"`

	// Args are passed to the entry command.
	Args []string `json:"args,omitempty"`

	// Env is a set of environment variables the runtime injects.
	Env map[string]string `json:"env,omitempty"`

	// HealthCheck is an HTTP path the host polls to verify readiness.
	HealthCheck string `json:"health_check,omitempty"`

	// Port is the local port exposed by the sidecar for HTTP callbacks.
	Port int `json:"port,omitempty"`

	// Image is the container image reference (for Kind == RuntimeContainer).
	Image string `json:"image,omitempty"`

	// RestartPolicy: "never" | "on-failure" | "always". Default "on-failure".
	RestartPolicy string `json:"restart_policy,omitempty"`

	// StartTimeoutSec caps how long the host waits for health-check success.
	// 0 means the runtime default (30s).
	StartTimeoutSec int `json:"start_timeout_sec,omitempty"`
}

// CogniRef points to a Cogni declaration shipped with the capsule, or an
// external Cogni ID registered by another capsule.
type CogniRef struct {
	// ID is the cogni identifier. Falls back to the capsule name if empty.
	ID string `json:"id,omitempty"`

	// File is a path (relative to the capsule install dir) to a JSON/YAML
	// cogni declaration. Leave empty if the cogni is provided programmatically
	// via Capsule.Exports().Cogni (see exports.go).
	File string `json:"file,omitempty"`
}

// Validate checks that the Capsule manifest has the minimum required fields
// and self-consistent configuration.
func (m *Manifest) Validate() error {
	if m == nil {
		return fmt.Errorf("manifest: nil")
	}
	if err := m.Manifest.Validate(); err != nil {
		return err
	}
	if m.Runtime != nil {
		if err := m.Runtime.Validate(); err != nil {
			return fmt.Errorf("runtime: %w", err)
		}
	}
	for i, d := range m.Dependencies {
		if strings.TrimSpace(d.Name) == "" {
			return fmt.Errorf("dependency[%d]: name is required", i)
		}
	}
	return nil
}

// Validate ensures the RuntimeSpec is self-consistent.
func (s *RuntimeSpec) Validate() error {
	switch s.Kind {
	case "", RuntimeInProcess:
		// In-process runtimes ignore the other fields.
	case RuntimeSidecar:
		if s.Entry == "" {
			return fmt.Errorf("sidecar: entry is required")
		}
	case RuntimeContainer:
		if s.Image == "" {
			return fmt.Errorf("container: image is required")
		}
	default:
		return fmt.Errorf("unknown runtime kind: %q", s.Kind)
	}
	switch s.RestartPolicy {
	case "", "never", "on-failure", "always":
	default:
		return fmt.Errorf("restart_policy must be never|on-failure|always")
	}
	return nil
}

// ResolvedKind returns the effective runtime kind, defaulting to in-process.
func (s *RuntimeSpec) ResolvedKind() RuntimeKind {
	if s == nil || s.Kind == "" {
		return RuntimeInProcess
	}
	return s.Kind
}

// LoadManifest reads and validates a Capsule manifest from a JSON file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveManifest writes a Capsule manifest to disk (pretty-printed JSON).
func SaveManifest(m *Manifest, path string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// Name returns the display name, falling back to the capsule ID.
func (m *Manifest) DisplayOrName() string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	return m.Name
}
