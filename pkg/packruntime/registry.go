package packruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const RegistryFileName = "installed.json"

type PackStatus string

const (
	PackStatusInstalled PackStatus = "installed"
	PackStatusEnabled   PackStatus = "enabled"
	PackStatusDisabled  PackStatus = "disabled"
)

type InstalledPack struct {
	Manifest        Manifest   `json:"manifest"`
	Status          PackStatus `json:"status"`
	Source          string     `json:"source,omitempty"`
	InstalledAt     time.Time  `json:"installedAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	PreviousVersion string     `json:"previousVersion,omitempty"`
}

type RegistrySnapshot struct {
	Version int             `json:"version"`
	Packs   []InstalledPack `json:"packs"`
}

type Registry struct {
	path     string
	snapshot RegistrySnapshot
	now      func() time.Time
}

func NewRegistry(root string) (*Registry, error) {
	if root == "" {
		return nil, fmt.Errorf("pack registry root is required")
	}
	registry := &Registry{path: filepath.Join(root, RegistryFileName), now: time.Now}
	if err := registry.load(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *Registry) List() []InstalledPack {
	out := append([]InstalledPack(nil), r.snapshot.Packs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.ID < out[j].Manifest.ID })
	return out
}

func (r *Registry) Enabled() []InstalledPack {
	var out []InstalledPack
	for _, pack := range r.List() {
		if pack.Status == PackStatusEnabled {
			out = append(out, pack)
		}
	}
	return out
}

func (r *Registry) Get(id string) (InstalledPack, bool) {
	for _, pack := range r.snapshot.Packs {
		if pack.Manifest.ID == id {
			return pack, true
		}
	}
	return InstalledPack{}, false
}

func (r *Registry) Install(manifest Manifest, source string) (InstalledPack, error) {
	if err := manifest.Validate(); err != nil {
		return InstalledPack{}, err
	}
	now := r.now().UTC()
	status := PackStatusDisabled
	if manifest.DefaultState == "enabled" {
		status = PackStatusEnabled
	}
	for i, pack := range r.snapshot.Packs {
		if pack.Manifest.ID == manifest.ID {
			pack.PreviousVersion = pack.Manifest.Version
			pack.Manifest = manifest
			pack.Source = source
			pack.UpdatedAt = now
			if manifest.DefaultState == "enabled" || pack.Status == PackStatusEnabled {
				pack.Status = PackStatusEnabled
			}
			r.snapshot.Packs[i] = pack
			r.snapshot.Version++
			return pack, r.save()
		}
	}
	pack := InstalledPack{Manifest: manifest, Status: status, Source: source, InstalledAt: now, UpdatedAt: now}
	r.snapshot.Packs = append(r.snapshot.Packs, pack)
	r.snapshot.Version++
	return pack, r.save()
}

func (r *Registry) Enable(id string) (InstalledPack, error) {
	return r.setStatus(id, PackStatusEnabled)
}
func (r *Registry) Disable(id string) (InstalledPack, error) {
	return r.setStatus(id, PackStatusDisabled)
}

func (r *Registry) Rollback(id string) (InstalledPack, error) {
	for i, pack := range r.snapshot.Packs {
		if pack.Manifest.ID != id {
			continue
		}
		if !pack.Manifest.Update.Rollback {
			return InstalledPack{}, fmt.Errorf("pack %q does not allow rollback", id)
		}
		if pack.PreviousVersion == "" {
			return InstalledPack{}, fmt.Errorf("pack %q has no previous version", id)
		}
		pack.Manifest.Version, pack.PreviousVersion = pack.PreviousVersion, pack.Manifest.Version
		pack.UpdatedAt = r.now().UTC()
		r.snapshot.Packs[i] = pack
		r.snapshot.Version++
		return pack, r.save()
	}
	return InstalledPack{}, fmt.Errorf("pack %q is not installed", id)
}

func (r *Registry) setStatus(id string, status PackStatus) (InstalledPack, error) {
	for i, pack := range r.snapshot.Packs {
		if pack.Manifest.ID != id {
			continue
		}
		pack.Status = status
		pack.UpdatedAt = r.now().UTC()
		r.snapshot.Packs[i] = pack
		r.snapshot.Version++
		return pack, r.save()
	}
	return InstalledPack{}, fmt.Errorf("pack %q is not installed", id)
}

func (r *Registry) load() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			r.snapshot = RegistrySnapshot{Version: 1, Packs: []InstalledPack{}}
			return nil
		}
		return fmt.Errorf("read pack registry: %w", err)
	}
	if err := json.Unmarshal(data, &r.snapshot); err != nil {
		return fmt.Errorf("parse pack registry: %w", err)
	}
	if r.snapshot.Version == 0 {
		r.snapshot.Version = 1
	}
	return nil
}

func (r *Registry) save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("create pack registry dir: %w", err)
	}
	data, err := json.MarshalIndent(r.snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pack registry: %w", err)
	}
	return os.WriteFile(r.path, append(data, '\n'), 0o644)
}
