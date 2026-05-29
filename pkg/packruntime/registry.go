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
	"sync"
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
	Manifest          Manifest       `json:"manifest"`
	Status            PackStatus     `json:"status"`
	Source            string         `json:"source,omitempty"`
	Artifacts         *PackArtifacts `json:"artifacts,omitempty"`
	PreviousArtifacts *PackArtifacts `json:"previousArtifacts,omitempty"`
	InstalledAt       time.Time      `json:"installedAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	PreviousVersion   string         `json:"previousVersion,omitempty"`
}

type PackArtifacts struct {
	PackagePath string    `json:"packagePath,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	SizeBytes   int64     `json:"sizeBytes,omitempty"`
	CachedAt    time.Time `json:"cachedAt"`
}

type PruneReport struct {
	Removed []string `json:"removed"`
	Kept    []string `json:"kept"`
	Errors  []string `json:"errors,omitempty"`
}

type RegistrySnapshot struct {
	Version int             `json:"version"`
	Packs   []InstalledPack `json:"packs"`
}

type ChangeReason string

const (
	ChangeReasonInstall  ChangeReason = "install"
	ChangeReasonUpdate   ChangeReason = "update"
	ChangeReasonEnable   ChangeReason = "enable"
	ChangeReasonDisable  ChangeReason = "disable"
	ChangeReasonRollback ChangeReason = "rollback"
)

// ChangeEvent is emitted after a registry mutation has been persisted. Runtime
// modules use it to bind pack enabled/disabled state to background workloads
// without polling installed.json.
type ChangeEvent struct {
	Pack           InstalledPack `json:"pack"`
	PreviousStatus PackStatus    `json:"previousStatus,omitempty"`
	Status         PackStatus    `json:"status"`
	Reason         ChangeReason  `json:"reason"`
}

type ChangeHook func(ChangeEvent)

type Registry struct {
	root     string
	path     string
	snapshot RegistrySnapshot
	now      func() time.Time
	hooksMu  sync.RWMutex
	hooks    []ChangeHook
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

// InstalledDir returns the on-disk extraction directory for a pack id/version,
// matching the layout InstallFromYqpack writes to:
// <root>/installed/<safeID>-<safeVersion>. It does not check existence.
func (r *Registry) InstalledDir(id, version string) string {
	return filepath.Join(r.root, "installed", safeArtifactSegment(id)+"-"+safeArtifactSegment(version))
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
			previousStatus := pack.Status
			pack.PreviousVersion = pack.Manifest.Version
			pack.PreviousArtifacts = clonePackArtifacts(pack.Artifacts)
			pack.Manifest = manifest
			pack.Source = source
			pack.Artifacts = clonePackArtifacts(artifacts)
			pack.UpdatedAt = now
			if manifest.DefaultState == "enabled" || pack.Status == PackStatusEnabled {
				pack.Status = PackStatusEnabled
			}
			r.snapshot.Packs[i] = pack
			r.snapshot.Version++
			if err := r.save(); err != nil {
				return pack, err
			}
			r.notify(ChangeEvent{Pack: cloneInstalledPack(pack), PreviousStatus: previousStatus, Status: pack.Status, Reason: ChangeReasonUpdate})
			return pack, nil
		}
	}
	pack := InstalledPack{Manifest: manifest, Status: status, Source: source, Artifacts: clonePackArtifacts(artifacts), InstalledAt: now, UpdatedAt: now}
	r.snapshot.Packs = append(r.snapshot.Packs, pack)
	r.snapshot.Version++
	if err := r.save(); err != nil {
		return pack, err
	}
	r.notify(ChangeEvent{Pack: cloneInstalledPack(pack), Status: pack.Status, Reason: ChangeReasonInstall})
	return pack, nil
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

func (r *Registry) PruneArtifacts() PruneReport {
	artifactsRoot := filepath.Join(r.root, "artifacts")
	report := PruneReport{Removed: []string{}, Kept: []string{}}
	referenced := r.referencedArtifactPaths()
	if _, err := os.Stat(artifactsRoot); err != nil {
		if os.IsNotExist(err) {
			return report
		}
		report.Errors = append(report.Errors, err.Error())
		return report
	}
	_ = filepath.WalkDir(artifactsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
			return nil
		}
		if d.IsDir() {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
			return nil
		}
		if referenced[abs] {
			report.Kept = append(report.Kept, path)
			return nil
		}
		if err := os.Remove(path); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("remove %s: %v", path, err))
			return nil
		}
		report.Removed = append(report.Removed, path)
		return nil
	})
	sort.Strings(report.Kept)
	sort.Strings(report.Removed)
	return report
}

func (r *Registry) referencedArtifactPaths() map[string]bool {
	referenced := make(map[string]bool)
	add := func(artifacts *PackArtifacts) {
		if artifacts == nil || strings.TrimSpace(artifacts.PackagePath) == "" {
			return
		}
		abs, err := filepath.Abs(artifacts.PackagePath)
		if err != nil {
			return
		}
		referenced[abs] = true
	}
	for _, pack := range r.snapshot.Packs {
		add(pack.Artifacts)
		add(pack.PreviousArtifacts)
	}
	return referenced
}

func clonePackArtifacts(artifacts *PackArtifacts) *PackArtifacts {
	if artifacts == nil {
		return nil
	}
	clone := *artifacts
	return &clone
}

func cloneInstalledPack(pack InstalledPack) InstalledPack {
	pack.Artifacts = clonePackArtifacts(pack.Artifacts)
	pack.PreviousArtifacts = clonePackArtifacts(pack.PreviousArtifacts)
	return pack
}

// OnChange registers a synchronous hook that runs after successful registry
// mutations. Hooks are intentionally process-local and are not serialized into
// installed.json.
func (r *Registry) OnChange(fn ChangeHook) {
	if r == nil || fn == nil {
		return
	}
	r.hooksMu.Lock()
	defer r.hooksMu.Unlock()
	r.hooks = append(r.hooks, fn)
}

func (r *Registry) notify(event ChangeEvent) {
	if r == nil {
		return
	}
	r.hooksMu.RLock()
	hooks := append([]ChangeHook(nil), r.hooks...)
	r.hooksMu.RUnlock()
	for _, hook := range hooks {
		hook(event)
	}
}

func (r *Registry) Enable(id string) (InstalledPack, error) {
	return r.setStatus(id, PackStatusEnabled, ChangeReasonEnable)
}
func (r *Registry) Disable(id string) (InstalledPack, error) {
	return r.setStatus(id, PackStatusDisabled, ChangeReasonDisable)
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
		pack.Artifacts, pack.PreviousArtifacts = clonePackArtifacts(pack.PreviousArtifacts), clonePackArtifacts(pack.Artifacts)
		pack.UpdatedAt = r.now().UTC()
		r.snapshot.Packs[i] = pack
		r.snapshot.Version++
		if err := r.save(); err != nil {
			return pack, err
		}
		r.notify(ChangeEvent{Pack: cloneInstalledPack(pack), PreviousStatus: pack.Status, Status: pack.Status, Reason: ChangeReasonRollback})
		return pack, nil
	}
	return InstalledPack{}, fmt.Errorf("pack %q is not installed", id)
}

func (r *Registry) setStatus(id string, status PackStatus, reason ChangeReason) (InstalledPack, error) {
	for i, pack := range r.snapshot.Packs {
		if pack.Manifest.ID != id {
			continue
		}
		previousStatus := pack.Status
		pack.Status = status
		pack.UpdatedAt = r.now().UTC()
		r.snapshot.Packs[i] = pack
		r.snapshot.Version++
		if err := r.save(); err != nil {
			return pack, err
		}
		r.notify(ChangeEvent{Pack: cloneInstalledPack(pack), PreviousStatus: previousStatus, Status: pack.Status, Reason: reason})
		return pack, nil
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
