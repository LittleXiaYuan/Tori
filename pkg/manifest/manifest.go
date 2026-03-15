package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// Manifest declares a plugin's metadata, version, permissions, and signature.
type Manifest struct {
	Name        string       `json:"name"`
	Version     string       `json:"version"`      // semver: "1.2.3"
	Description string       `json:"description"`
	Author      string       `json:"author"`
	License     string       `json:"license,omitempty"`
	MinAgent    string       `json:"min_agent,omitempty"` // minimum agent version
	Permissions []Permission `json:"permissions"`
	Signature   string       `json:"signature,omitempty"` // SHA256 of plugin binary/source
	Skills      []SkillDecl  `json:"skills"`
}

// Permission declares a capability the plugin requires.
type Permission struct {
	Name        string `json:"name"`        // e.g. "network", "filesystem", "sandbox", "llm"
	Description string `json:"description"` // human-readable reason
	Required    bool   `json:"required"`    // false = optional, graceful degradation
}

// SkillDecl declares a skill provided by the plugin.
type SkillDecl struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Dangerous   bool   `json:"dangerous,omitempty"` // requires user confirmation before execution
}

// Well-known permission names.
const (
	PermNetwork    = "network"     // outbound HTTP/TCP
	PermFilesystem = "filesystem"  // host file read/write
	PermSandbox    = "sandbox"     // code execution
	PermLLM        = "llm"         // LLM API calls
	PermDatabase   = "database"    // database access
	PermSecrets    = "secrets"     // access to env vars / secrets
)

// Validate checks that the manifest has required fields and valid semver.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("manifest: version is required")
	}
	if !isValidSemver(m.Version) {
		return fmt.Errorf("manifest: version %q is not valid semver", m.Version)
	}
	for _, p := range m.Permissions {
		if p.Name == "" {
			return fmt.Errorf("manifest: permission name is required")
		}
	}
	return nil
}

// HasPermission checks if the manifest declares a specific permission.
func (m *Manifest) HasPermission(name string) bool {
	for _, p := range m.Permissions {
		if p.Name == name {
			return true
		}
	}
	return false
}

// RequiredPermissions returns only the required permissions.
func (m *Manifest) RequiredPermissions() []Permission {
	var out []Permission
	for _, p := range m.Permissions {
		if p.Required {
			out = append(out, p)
		}
	}
	return out
}

// ComputeSignature generates a SHA256 hash of the given file for integrity verification.
func ComputeSignature(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file for signature: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// VerifySignature checks if the file matches the expected signature.
func VerifySignature(filePath, expected string) (bool, error) {
	actual, err := ComputeSignature(filePath)
	if err != nil {
		return false, err
	}
	return actual == expected, nil
}

// LoadFromFile reads a manifest from a JSON file.
func LoadFromFile(path string) (*Manifest, error) {
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

// SaveToFile writes the manifest to a JSON file.
func (m *Manifest) SaveToFile(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// isValidSemver does a basic check for "major.minor.patch" format.
func isValidSemver(v string) bool {
	parts := 0
	for _, c := range v {
		if c == '.' {
			parts++
		} else if c < '0' || c > '9' {
			// Allow pre-release suffix like -alpha, -beta
			if c == '-' && parts >= 2 {
				return true
			}
			return false
		}
	}
	return parts == 2
}
