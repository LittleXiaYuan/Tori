// .yqpack container: a deterministic ZIP carrying a pack manifest plus
// optional backend binaries / frontend assets / sdk slices.
//
// Determinism guarantees (per docs/spec/pack-distribution-spec.md §2.1):
//   - entries written in lexicographic path order
//   - timestamps fixed at 1980-01-01T00:00:00Z (the ZIP epoch)
//   - no ZIP comment, no extra fields
//   - identical inputs produce byte-identical archives
package packruntime

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ManifestSigName is the detached signature file inside a .yqpack.
const ManifestSigName = "manifest.sig"

// ManifestPubName is the publisher reference file inside a .yqpack. It does
// NOT contain key bytes — only "<publisherID>:<publicKeyID>".
const ManifestPubName = "manifest.pub"

// zipEpoch is the fixed timestamp written to every entry. ZIP can't represent
// times before 1980-01-01.
var zipEpoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// PackToYqpack writes a deterministic .yqpack archive at out from the
// contents of srcDir. srcDir must contain pack.json at its root. All files
// under srcDir (recursive) are included; .yqpack files inside srcDir are
// skipped to allow re-running the tool against its own output dir.
//
// Returns the SHA256 of the resulting archive.
func PackToYqpack(srcDir, out string) (string, error) {
	manifestPath := filepath.Join(srcDir, ManifestFileName)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return "", fmt.Errorf("yqpack: load manifest: %w", err)
	}
	if strings.TrimSpace(manifest.ID) == "" || strings.TrimSpace(manifest.Version) == "" {
		return "", fmt.Errorf("yqpack: manifest must declare id and version")
	}

	files, err := collectYqpackFiles(srcDir)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", fmt.Errorf("yqpack: create out dir: %w", err)
	}
	tmp := out + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("yqpack: create %s: %w", tmp, err)
	}
	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)
	zw := zip.NewWriter(mw)

	for _, file := range files {
		if err := writeYqpackEntry(zw, srcDir, file); err != nil {
			zw.Close()
			f.Close()
			_ = os.Remove(tmp)
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return "", fmt.Errorf("yqpack: close zip: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("yqpack: close file: %w", err)
	}
	if err := os.Rename(tmp, out); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("yqpack: rename %s -> %s: %w", tmp, out, err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func collectYqpackFiles(srcDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(rel, ".yqpack") || strings.HasSuffix(rel, ".yqpack.tmp") {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("yqpack: walk %s: %w", srcDir, err)
	}
	sort.Strings(files)
	return files, nil
}

func writeYqpackEntry(zw *zip.Writer, srcDir, rel string) error {
	src := filepath.Join(srcDir, rel)
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("yqpack: read %s: %w", src, err)
	}
	header := &zip.FileHeader{
		Name:     rel,
		Method:   zip.Deflate,
		Modified: zipEpoch,
	}
	header.SetMode(0o644)
	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("yqpack: header %s: %w", rel, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("yqpack: write %s: %w", rel, err)
	}
	return nil
}

// InstallOptions controls the optional verification steps performed by
// InstallFromYqpack. Zero value is the safe default: signature verification
// is on iff the manifest carries a signing block, and SHA256 is checked
// against ExpectedSHA256 only if it's non-empty.
type InstallOptions struct {
	// ExpectedSHA256 is the artifact SHA256 the caller expects. Empty means
	// "skip artifact-level check" (used when the caller already verified).
	ExpectedSHA256 string

	// TrustRoot resolves publisher public keys when the manifest is signed.
	// nil means "best-effort": signed manifests fail closed; unsigned manifests
	// pass (matches the dev-time pack.json direct-install path in the spec).
	TrustRoot PublicKeyResolver

	// AllowUnsigned skips signature verification even when the manifest
	// carries a signing block. Set only for development pulls.
	AllowUnsigned bool

	// Source string recorded into installed.json (e.g. "yqpack:/abs/path").
	Source string
}

// InstallFromYqpack opens a local .yqpack file, verifies it, extracts it to
// <root>/installed/<id>-<version>/, and registers the result via
// Registry.InstallWithArtifacts.
//
// Failure semantics (spec §4.4): nothing is written to installed.json unless
// every check passes. Failed staging directories are removed.
func (r *Registry) InstallFromYqpack(path string, opts InstallOptions) (InstalledPack, error) {
	if r == nil {
		return InstalledPack{}, fmt.Errorf("yqpack: nil registry")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return InstalledPack{}, fmt.Errorf("yqpack: abs path: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return InstalledPack{}, fmt.Errorf("yqpack: read %s: %w", abs, err)
	}
	digest := hex.EncodeToString(sha256Sum(data))
	if expected := normalizeSHA256(opts.ExpectedSHA256); expected != "" && !strings.EqualFold(expected, digest) {
		return InstalledPack{}, fmt.Errorf("yqpack: sha256 mismatch (expected %s got %s)", expected, digest)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return InstalledPack{}, fmt.Errorf("yqpack: open zip: %w", err)
	}

	manifestRaw, err := readZipFile(zr, ManifestFileName)
	if err != nil {
		return InstalledPack{}, fmt.Errorf("yqpack: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return InstalledPack{}, fmt.Errorf("yqpack: parse pack.json: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return InstalledPack{}, fmt.Errorf("yqpack: validate pack.json: %w", err)
	}

	if manifest.Signing != nil && !opts.AllowUnsigned {
		if err := VerifyManifest(manifest, opts.TrustRoot); err != nil {
			return InstalledPack{}, fmt.Errorf("yqpack: verify signature: %w", err)
		}
	}

	stagingRoot := filepath.Join(r.root, "staging")
	id := safeArtifactSegment(manifest.ID)
	version := safeArtifactSegment(manifest.Version)
	staging := filepath.Join(stagingRoot, id+"-"+version+"-"+digest[:8])
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return InstalledPack{}, fmt.Errorf("yqpack: mkdir staging: %w", err)
	}

	if err := extractZip(zr, staging); err != nil {
		_ = os.RemoveAll(staging)
		return InstalledPack{}, fmt.Errorf("yqpack: extract: %w", err)
	}

	installedRoot := filepath.Join(r.root, "installed")
	target := filepath.Join(installedRoot, id+"-"+version)
	if err := os.MkdirAll(installedRoot, 0o755); err != nil {
		_ = os.RemoveAll(staging)
		return InstalledPack{}, fmt.Errorf("yqpack: mkdir installed: %w", err)
	}
	_ = os.RemoveAll(target)
	if err := os.Rename(staging, target); err != nil {
		_ = os.RemoveAll(staging)
		return InstalledPack{}, fmt.Errorf("yqpack: install rename: %w", err)
	}

	artifacts := &PackArtifacts{
		PackagePath: abs,
		SHA256:      digest,
		SizeBytes:   int64(len(data)),
		CachedAt:    r.now().UTC(),
	}
	source := opts.Source
	if source == "" {
		source = "yqpack:" + abs
	}
	return r.InstallWithArtifacts(manifest, source, artifacts)
}

// InspectYqpackManifestFile reads only the pack manifest from a local .yqpack
// and returns the validated manifest plus the artifact SHA256 and byte size.
// It performs no signature verification and does not extract or install
// anything, making it suitable for catalog/preview UI flows.
func InspectYqpackManifestFile(path string) (Manifest, string, int64, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Manifest{}, "", 0, fmt.Errorf("yqpack: abs path: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Manifest{}, "", 0, fmt.Errorf("yqpack: read %s: %w", abs, err)
	}
	manifest, digest, err := InspectYqpackManifestBytes(data)
	return manifest, digest, int64(len(data)), err
}

// InspectYqpackFile returns a read-only Pack Studio report for a local .yqpack.
// It lists archive entries and builds a conservative modification plan without
// extracting, installing, signing, or mutating registry state.
func InspectYqpackFile(path string, expectedSHA256 string, goal string) (YqpackInspectReport, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return YqpackInspectReport{}, fmt.Errorf("yqpack: abs path: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return YqpackInspectReport{}, fmt.Errorf("yqpack: read %s: %w", abs, err)
	}
	return InspectYqpackBytes(data, abs, expectedSHA256, goal)
}

// InspectYqpackBytes is the byte-oriented counterpart to InspectYqpackFile. It
// is safe for downloaded package previews because it never writes file content
// to disk and never registers the pack.
func InspectYqpackBytes(data []byte, source string, expectedSHA256 string, goal string) (YqpackInspectReport, error) {
	digest := hex.EncodeToString(sha256Sum(data))
	expected := normalizeSHA256(expectedSHA256)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return YqpackInspectReport{}, fmt.Errorf("yqpack: open zip: %w", err)
	}
	manifestRaw, err := readZipFile(zr, ManifestFileName)
	if err != nil {
		return YqpackInspectReport{}, fmt.Errorf("yqpack: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return YqpackInspectReport{}, fmt.Errorf("yqpack: parse pack.json: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return YqpackInspectReport{}, fmt.Errorf("yqpack: validate pack.json: %w", err)
	}
	report := YqpackInspectReport{
		GeneratedAt:    time.Now().UTC(),
		Source:         strings.TrimSpace(source),
		SHA256:         digest,
		ExpectedSHA256: expected,
		SHA256Match:    expected == "" || strings.EqualFold(expected, digest),
		SizeBytes:      int64(len(data)),
		Manifest:       manifest,
		Entries:        []YqpackEntryReport{},
		Warnings:       []string{},
		Plan: BuildPackStudioPlan(manifest, PackStudioPlanOptions{
			Goal:      goal,
			Source:    "yqpack:" + strings.TrimSpace(source),
			Installed: false,
			Enabled:   false,
			Status:    "artifact",
		}),
	}
	if expected != "" && !report.SHA256Match {
		report.Warnings = append(report.Warnings, fmt.Sprintf("sha256 mismatch: expected %s got %s", expected, digest))
	}
	for _, f := range zr.File {
		entry := classifyYqpackEntry(f)
		report.Entries = append(report.Entries, entry)
		report.EntryCount++
		if entry.Editable {
			report.EditableCount++
		} else {
			report.GuardedCount++
		}
		if strings.Contains(entry.Path, "..") {
			report.Warnings = append(report.Warnings, "archive contains unsafe relative path: "+entry.Path)
		}
	}
	sort.Slice(report.Entries, func(i, j int) bool {
		return report.Entries[i].Path < report.Entries[j].Path
	})
	return report, nil
}

// InspectYqpackManifestBytes reads only pack.json from .yqpack bytes and
// returns the validated manifest plus the artifact SHA256. It performs no
// signature verification and does not extract or install anything.
func InspectYqpackManifestBytes(data []byte) (Manifest, string, error) {
	digest := hex.EncodeToString(sha256Sum(data))
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Manifest{}, digest, fmt.Errorf("yqpack: open zip: %w", err)
	}
	manifestRaw, err := readZipFile(zr, ManifestFileName)
	if err != nil {
		return Manifest{}, digest, fmt.Errorf("yqpack: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return Manifest{}, digest, fmt.Errorf("yqpack: parse pack.json: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, digest, fmt.Errorf("yqpack: validate pack.json: %w", err)
	}
	return manifest, digest, nil
}

func classifyYqpackEntry(f *zip.File) YqpackEntryReport {
	path := filepath.ToSlash(strings.TrimSpace(f.Name))
	entry := YqpackEntryReport{
		Path:      path,
		Kind:      "file",
		SizeBytes: int64(f.UncompressedSize64),
		Editable:  false,
		Reason:    "需要源码或对应运行时测试后才能修改。",
	}
	if f.FileInfo().IsDir() {
		entry.Kind = "directory"
		entry.Reason = "目录项本身不需要修改。"
		return entry
	}
	lower := strings.ToLower(path)
	switch {
	case lower == ManifestFileName:
		entry.Kind = "manifest"
		entry.Editable = true
		entry.Reason = "能力包 manifest，可改用途、入口、权限说明和发行元数据。"
	case lower == ManifestSigName || lower == ManifestPubName || strings.Contains(lower, "signature"):
		entry.Kind = "signature"
		entry.NeedsSource = true
		entry.Reason = "签名/公钥材料不能手改；改包后必须重新签名。"
	case strings.HasSuffix(lower, ".wasm"):
		entry.Kind = "wasm"
		entry.NeedsSource = true
		entry.Reason = "WASM 二进制不能硬改；需要源码、ABI 说明和 wasm 回归测试。"
	case strings.HasPrefix(lower, "frontend/") && (strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".css") || strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".json")):
		entry.Kind = "frontend"
		entry.Editable = true
		entry.Reason = "iframe/DLC 前端资源，可在沙箱边界内优化界面和文案。"
	case strings.HasSuffix(lower, ".md") || strings.HasPrefix(lower, "docs/") || strings.HasPrefix(lower, "examples/"):
		entry.Kind = "docs"
		entry.Editable = true
		entry.Reason = "说明、示例和文档可直接改善用户理解。"
	case strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") || strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".py"):
		entry.Kind = "source"
		entry.Editable = true
		entry.Reason = "源码可改，但必须配套测试和重新打包。"
	case strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".dll") || strings.HasSuffix(lower, ".so") || strings.HasSuffix(lower, ".dylib") || strings.HasSuffix(lower, ".bin"):
		entry.Kind = "binary"
		entry.NeedsSource = true
		entry.Reason = "二进制文件不能直接改造；需要源码或上游构建产物。"
	default:
		entry.Kind = "asset"
		entry.Reason = "资产或运行时文件，修改前需要确认引用关系和测试覆盖。"
	}
	return entry
}

func sha256Sum(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open %s: %w", name, err)
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("missing entry %s", name)
}

func extractZip(zr *zip.Reader, dest string) error {
	for _, f := range zr.File {
		if strings.Contains(f.Name, "..") {
			return fmt.Errorf("zip slip: %s", f.Name)
		}
		clean := filepath.Clean(f.Name)
		path := filepath.Join(dest, clean)
		rel, err := filepath.Rel(dest, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("zip slip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(path)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(w, rc); err != nil {
			rc.Close()
			w.Close()
			return err
		}
		rc.Close()
		if err := w.Close(); err != nil {
			return err
		}
	}
	return nil
}
