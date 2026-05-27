// Package backuppack contains the backend implementation for the built-in
// backup capability pack. Gateway only gates and mounts these handlers; the
// backup business logic lives here so it can keep moving out of the monolith.
package backuppack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/appdir"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/version"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.backup"

// Manifest describes the content of a backup archive.
type Manifest struct {
	AgentVersion    string            `json:"agent_version"`
	ExportedAt      time.Time         `json:"exported_at"`
	BackupType      string            `json:"backup_type"` // "manual" or "auto"
	ManifestVersion string            `json:"manifest_version"`
	Files           map[string]string `json:"files"` // relative path → sha256
	FileSizes       map[string]int64  `json:"file_sizes"`
}

// Config describes the runtime dependencies for backup pack handlers.
type Config struct {
	DataDir string
	Version string
	Now     func() time.Time
}

// Handler serves the backup pack API surface.
type Handler struct {
	dataDir string
	version string
	now     func() time.Time
}

// New creates a backup pack handler.
func New(cfg Config) *Handler {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = appdir.DataDir()
	}
	agentVersion := cfg.Version
	if agentVersion == "" {
		agentVersion = version.Version
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{dataDir: dataDir, version: agentVersion, now: now}
}

// DefaultHandler returns a handler bound to the current app data directory and
// build version. It keeps legacy gateway wrappers tiny while the pack boundary
// is being extracted.
func DefaultHandler() *Handler {
	return New(Config{})
}

// PackID returns the stable manifest id for the built-in backup pack.
func (h *Handler) PackID() string {
	return PackID
}

// Routes exposes the backup pack HTTP API to the Pack Runtime host.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/backup/info", Handler: h.Info},
		{Method: http.MethodGet, Path: "/v1/backup/export", Handler: h.Export},
		{Method: http.MethodPost, Path: "/v1/backup/import", Handler: h.Import},
	}
}

var relFiles = []string{
	"memory.json",
	"adaptive.json",
	"graph.json",
	"editable.json",
	"audit.jsonl",
	"mcp.json",
	"cron/jobs.json",
	"persona/IDENTITY.md",
	"persona/SOUL.md",
}

var relDirs = []string{
	"sessions",
	"persona/skills",
	"plugins",
	"ledger",
}

// Export creates a ZIP backup and streams it to the client.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stamp := h.now().Format("20060102-150405")
	filename := fmt.Sprintf("yunque-backup-%s.zip", stamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	zw := zip.NewWriter(w)
	defer zw.Close()

	manifest := Manifest{
		AgentVersion:    h.version,
		ExportedAt:      h.now().UTC(),
		BackupType:      "manual",
		ManifestVersion: "1.0",
		Files:           make(map[string]string),
		FileSizes:       make(map[string]int64),
	}

	for _, abs := range h.backupFiles() {
		rel, _ := filepath.Rel(h.dataDir, abs)
		if err := addFileToZip(zw, abs, filepath.ToSlash(rel), &manifest); err != nil {
			slog.Debug("backup: skip file", "path", abs, "err", err)
		}
	}

	for _, dir := range h.backupDirs() {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(h.dataDir, path)
				return addFileToZip(zw, path, filepath.ToSlash(rel), &manifest)
			})
		}
	}

	mdata, _ := json.MarshalIndent(manifest, "", "  ")
	mw, _ := zw.Create("manifest.json")
	mw.Write(mdata)

	slog.Info("backup exported", "files", len(manifest.Files), "filename", filename)
}

// Import restores from an uploaded ZIP backup.
func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)

	file, _, err := r.FormFile("backup")
	if err != nil {
		http.Error(w, "missing backup file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmpFile, err := os.CreateTemp("", "yunque-restore-*.zip")
	if err != nil {
		http.Error(w, "failed to create temp file", http.StatusInternalServerError)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	written, err := io.Copy(tmpFile, file)
	tmpFile.Close()
	if err != nil {
		http.Error(w, "failed to read upload", http.StatusInternalServerError)
		return
	}

	zr, err := zip.OpenReader(tmpPath)
	if err != nil {
		http.Error(w, "invalid zip archive", http.StatusBadRequest)
		return
	}
	defer zr.Close()

	manifest, ok := readManifest(&zr.Reader)
	if !ok {
		http.Error(w, "backup archive missing manifest.json", http.StatusBadRequest)
		return
	}

	versionWarning := ""
	if manifest.AgentVersion != h.version {
		backupMajor := majorVersion(manifest.AgentVersion)
		currentMajor := majorVersion(h.version)
		if backupMajor != currentMajor {
			apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("major version mismatch: backup=%s current=%s", manifest.AgentVersion, h.version))
			return
		}
		versionWarning = fmt.Sprintf("minor version difference: backup=%s current=%s", manifest.AgentVersion, h.version)
	}

	if err := validateEntries(&zr.Reader, manifest); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	restored := h.restoreEntries(&zr.Reader, manifest)

	slog.Info("backup restored", "files", restored, "from_version", manifest.AgentVersion, "size_bytes", written)

	resp := map[string]any{
		"status":         "restored",
		"files_restored": restored,
		"from_version":   manifest.AgentVersion,
		"size_bytes":     written,
	}
	if versionWarning != "" {
		resp["warning"] = versionWarning
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Info returns information about what would be backed up.
func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files := make(map[string]int64)

	for _, abs := range h.backupFiles() {
		if info, err := os.Stat(abs); err == nil {
			rel, _ := filepath.Rel(h.dataDir, abs)
			files[filepath.ToSlash(rel)] = info.Size()
		}
	}

	for _, dir := range h.backupDirs() {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(h.dataDir, path)
			files[filepath.ToSlash(rel)] = info.Size()
			return nil
		})
	}

	var total int64
	for _, sz := range files {
		total += sz
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"files":       files,
		"file_count":  len(files),
		"total_bytes": total,
		"version":     h.version,
	})
}

func (h *Handler) backupFiles() []string {
	out := make([]string, len(relFiles))
	for i, f := range relFiles {
		out[i] = filepath.Join(h.dataDir, f)
	}
	return out
}

func (h *Handler) backupDirs() []string {
	out := make([]string, len(relDirs))
	for i, d := range relDirs {
		out[i] = filepath.Join(h.dataDir, d)
	}
	return out
}

func readManifest(zr *zip.Reader) (*Manifest, bool) {
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, false
		}
		defer rc.Close()
		var m Manifest
		if err := json.NewDecoder(rc).Decode(&m); err != nil {
			return nil, false
		}
		return &m, true
	}
	return nil, false
}

func validateEntries(zr *zip.Reader, manifest *Manifest) error {
	for _, f := range zr.File {
		if f.Name == "manifest.json" || f.FileInfo().IsDir() {
			continue
		}
		cleanName := filepath.Clean(filepath.FromSlash(f.Name))
		if strings.Contains(cleanName, "..") {
			return fmt.Errorf("backup archive contains invalid traversal path")
		}
		expected, ok := manifest.Files[f.Name]
		if !ok || expected == "" {
			return fmt.Errorf("unexpected file in backup: %s", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to read backup entry: %s", f.Name)
		}
		h := sha256.New()
		if _, err := io.Copy(h, rc); err != nil {
			rc.Close()
			return fmt.Errorf("failed to validate backup entry: %s", f.Name)
		}
		rc.Close()
		actualHash := hex.EncodeToString(h.Sum(nil))
		if actualHash != expected {
			return fmt.Errorf("checksum mismatch for %s", f.Name)
		}
	}
	return nil
}

func (h *Handler) restoreEntries(zr *zip.Reader, manifest *Manifest) int {
	restored := 0
	for _, f := range zr.File {
		if f.Name == "manifest.json" {
			continue
		}

		cleanName := filepath.Clean(filepath.FromSlash(f.Name))
		if strings.Contains(cleanName, "..") {
			slog.Warn("backup: skip traversal path", "path", f.Name)
			continue
		}
		if _, ok := manifest.Files[f.Name]; !ok {
			slog.Warn("backup: skip undeclared file", "file", f.Name)
			continue
		}

		targetPath := filepath.Join(h.dataDir, cleanName)
		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0o755)
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		os.MkdirAll(filepath.Dir(targetPath), 0o755)
		outFile, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			continue
		}

		_, copyErr := io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if copyErr != nil {
			slog.Warn("backup: failed to restore file", "file", f.Name, "err", copyErr)
			continue
		}
		restored++
	}
	return restored
}

func addFileToZip(zw *zip.Writer, absPath, archiveName string, manifest *Manifest) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	h := sha256.Sum256(data)
	manifest.Files[archiveName] = hex.EncodeToString(h[:])
	manifest.FileSizes[archiveName] = int64(len(data))
	w, err := zw.Create(archiveName)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func majorVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}
