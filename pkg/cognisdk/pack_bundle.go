package cognisdk

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const currentPackBundleVersion = 1

// NewPackBundle builds a portable bundle from pack manifests. Enabled pack IDs
// are optional; when omitted, hosts may enable all packs after loading.
func NewPackBundle(id string, packs []PackManifest, enabled []string) (PackBundle, error) {
	bundle := PackBundle{
		Version:      currentPackBundleVersion,
		ID:           strings.TrimSpace(id),
		CreatedAt:    time.Now().UTC(),
		Packs:        append([]PackManifest(nil), packs...),
		EnabledPacks: appendUnique(nil, enabled...),
	}
	if bundle.ID == "" {
		bundle.ID = "cogni-pack-bundle"
	}
	if err := ValidatePackBundle(bundle); err != nil {
		return PackBundle{}, err
	}
	return bundle, nil
}

// ExportBundle returns a deterministic bundle snapshot for the manager.
func (pm *PackManager) ExportBundle(id string) (PackBundle, error) {
	if pm == nil {
		return NewPackBundle(id, nil, nil)
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	ids := make([]string, 0, len(pm.packs))
	for id := range pm.packs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	packs := make([]PackManifest, 0, len(ids))
	enabled := make([]string, 0, len(ids))
	for _, packID := range ids {
		packs = append(packs, pm.packs[packID])
		if pm.enabled[packID] {
			enabled = append(enabled, packID)
		}
	}
	return NewPackBundle(id, packs, enabled)
}

// NewPackManagerFromBundle validates a portable bundle and restores its enabled
// pack set. If EnabledPacks is empty, all packs remain enabled by default.
func NewPackManagerFromBundle(bundle PackBundle) (*PackManager, error) {
	if err := ValidatePackBundle(bundle); err != nil {
		return nil, err
	}
	pm := NewPackManager(bundle.Packs...)
	if len(bundle.EnabledPacks) == 0 {
		return pm, nil
	}
	for _, status := range pm.List() {
		_ = pm.Disable(status.ID)
	}
	for _, id := range bundle.EnabledPacks {
		if err := pm.Enable(id); err != nil {
			return nil, err
		}
	}
	return pm, nil
}

// SavePackBundle writes a bundle as pretty JSON for sharing or committing as a
// small increment package artifact.
func SavePackBundle(bundle PackBundle, path string) error {
	if err := ValidatePackBundle(bundle); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("cognisdk.bundle: marshal %q: %w", bundle.ID, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cognisdk.bundle: write %q: %w", path, err)
	}
	return nil
}

// LoadPackBundle reads a portable bundle JSON file and validates every pack.
func LoadPackBundle(path string) (*PackBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cognisdk.bundle: read %q: %w", path, err)
	}
	var bundle PackBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("cognisdk.bundle: parse json %q: %w", path, err)
	}
	if err := ValidatePackBundle(bundle); err != nil {
		return nil, fmt.Errorf("cognisdk.bundle: validate %q: %w", path, err)
	}
	return &bundle, nil
}

// ValidatePackBundle checks the bundle envelope and every contained manifest.
func ValidatePackBundle(bundle PackBundle) error {
	if bundle.Version != currentPackBundleVersion {
		return fmt.Errorf("cognisdk.bundle %q: unsupported version %d", bundle.ID, bundle.Version)
	}
	if strings.TrimSpace(bundle.ID) == "" {
		return fmt.Errorf("cognisdk.bundle: id is required")
	}
	seen := make(map[string]bool, len(bundle.Packs))
	for _, pack := range bundle.Packs {
		if err := ValidatePack(pack); err != nil {
			return err
		}
		if seen[pack.ID] {
			return fmt.Errorf("cognisdk.bundle %q: duplicate pack %q", bundle.ID, pack.ID)
		}
		seen[pack.ID] = true
	}
	for _, id := range bundle.EnabledPacks {
		if !seen[id] {
			return fmt.Errorf("cognisdk.bundle %q: enabled pack %q not found", bundle.ID, id)
		}
	}
	return nil
}
