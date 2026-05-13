package packruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	Manifest        Manifest       `json:"manifest"`
	Status          PackStatus     `json:"status"`
	Source          string         `json:"source,omitempty"`
	Artifacts       *PackArtifacts `json:"artifacts,omitempty"`
	InstalledAt     time.Time      `json:"installedAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	PreviousVersion string         `json:"previousVersion,omitempty"`
}

type PackArtifacts struct {
	PackagePath string    `json:"packagePath,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	SizeBytes   int64     `json:"sizeBytes,omitempty"`
	CachedAt    time.Time `json:"cachedAt"`
}

type RegistrySnapshot struct {
	Version int             `json:"version"`
	Packs   []InstalledPack `json:"packs"`
}

type Registry struct {
	root     string
	path     string
	snapshot RegistrySnapshot
	now      func() time.Time
}

func NewRegistry(root string) (*Registry, error) {
	if root == "" {
		return nil, fmt.Errorf("pack registry root is required")
	}
	registry := &Registry{root: root, path: filepath.Join(root, RegistryFileName), now: time.Now}
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
	return r.InstallWithArtifacts(manifest, source, nil)
}

func (r *Registry) InstallWithArtifacts(manifest Manifest, source string, artifacts *PackArtifacts) (InstalledPack, error) {
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
			pack.Artifacts = artifacts
			pack.UpdatedAt = now
			if manifest.DefaultState == "enabled" || pack.Status == PackStatusEnabled {
				pack.Status = PackStatusEnabled
			}
			r.snapshot.Packs[i] = pack
			r.snapshot.Version++
			return pack, r.save()
		}
	}
	pack := InstalledPack{Manifest: manifest, Status: status, Source: source, Artifacts: artifacts, InstalledAt: now, UpdatedAt: now}
	r.snapshot.Packs = append(r.snapshot.Packs, pack)
	r.snapshot.Version++
	return pack, r.save()
}

func (r *Registry) CacheDistribution(ctx context.Context, manifest Manifest) (*PackArtifacts, error) {
	packageURL := strings.TrimSpace(manifest.Distribution.PackageURL)
	if packageURL == "" {
		return nil, fmt.Errorf("distribution.packageUrl is required for download")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, packageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create pack package request: %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download pack package: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("download pack package: http %d", res.StatusCode)
	}
	dir := filepath.Join(r.root, "artifacts", safeArtifactSegment(manifest.ID), safeArtifactSegment(manifest.Version))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create pack artifact dir: %w", err)
	}
	fileName := artifactFileName(packageURL)
	target := filepath.Join(dir, fileName)
	tmp := target + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("create pack artifact file: %w", err)
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), res.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("write pack artifact file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("close pack artifact file: %w", closeErr)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	expected := normalizeSHA256(manifest.Distribution.SHA256)
	if expected != "" && !strings.EqualFold(actual, expected) {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("pack artifact sha256 mismatch: expected %s got %s", expected, actual)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("commit pack artifact file: %w", err)
	}
	return &PackArtifacts{PackagePath: target, SHA256: actual, SizeBytes: size, CachedAt: r.now().UTC()}, nil
}

func normalizeSHA256(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.ToLower(value), "sha256:")
	return value
}

func artifactFileName(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if base := filepath.Base(parsed.Path); base != "." && base != "/" && base != "" {
			return safeArtifactSegment(base)
		}
	}
	return "package.tgz"
}

func safeArtifactSegment(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		return "pack"
	}
	return out
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
