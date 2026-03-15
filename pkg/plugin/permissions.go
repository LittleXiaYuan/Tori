package plugin

import (
	"fmt"
	"log/slog"

	"yunque-agent/pkg/manifest"
)

// PermissionPolicy defines which permissions are granted to plugins.
type PermissionPolicy struct {
	// Allowed is the set of permission names that are granted.
	// If empty, all permissions are granted (permissive mode).
	Allowed map[string]bool
	// Strict mode: reject plugins with undeclared required permissions.
	Strict bool
}

// DefaultPolicy returns a permissive policy (all permissions granted).
func DefaultPolicy() *PermissionPolicy {
	return &PermissionPolicy{Allowed: nil, Strict: false}
}

// RestrictedPolicy returns a policy with only specific permissions allowed.
func RestrictedPolicy(perms ...string) *PermissionPolicy {
	allowed := make(map[string]bool, len(perms))
	for _, p := range perms {
		allowed[p] = true
	}
	return &PermissionPolicy{Allowed: allowed, Strict: true}
}

// CheckManifest validates that a plugin's manifest permissions are satisfiable
// under this policy. Returns an error if required permissions are denied.
func (pp *PermissionPolicy) CheckManifest(m *manifest.Manifest) error {
	if pp.Allowed == nil {
		return nil // permissive mode
	}

	for _, perm := range m.Permissions {
		if !perm.Required {
			continue
		}
		if !pp.Allowed[perm.Name] {
			if pp.Strict {
				return fmt.Errorf("plugin %q requires permission %q which is not granted", m.Name, perm.Name)
			}
			slog.Warn("plugin permission not granted (non-strict)",
				"plugin", m.Name, "permission", perm.Name)
		}
	}
	return nil
}

// CheckPermission checks if a specific permission is granted at runtime.
func (pp *PermissionPolicy) CheckPermission(permName string) bool {
	if pp.Allowed == nil {
		return true // permissive
	}
	return pp.Allowed[permName]
}

// VerifySignature checks a plugin's manifest signature against its binary.
// Returns nil if signature matches, skip, or not set.
func VerifyPluginSignature(m *manifest.Manifest, binaryPath string) error {
	if m.Signature == "" {
		slog.Warn("plugin has no signature", "plugin", m.Name)
		return nil // no signature to verify
	}
	if binaryPath == "" {
		return nil // library plugin, no binary
	}

	ok, err := manifest.VerifySignature(binaryPath, m.Signature)
	if err != nil {
		return fmt.Errorf("signature verification failed for %q: %w", m.Name, err)
	}
	if !ok {
		return fmt.Errorf("signature mismatch for plugin %q: binary has been tampered with", m.Name)
	}
	slog.Info("plugin signature verified", "plugin", m.Name)
	return nil
}
