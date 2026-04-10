package gateway

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

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/appdir"
	"yunque-agent/internal/version"
)

// BackupManifest describes the content of a backup archive.
type BackupManifest struct {
	AgentVersion    string            `json:"agent_version"`
	ExportedAt      time.Time         `json:"exported_at"`
	BackupType      string            `json:"backup_type"` // "manual" or "auto"
	ManifestVersion string            `json:"manifest_version"`
	Files           map[string]string `json:"files"` // relative path → sha256
	FileSizes       map[string]int64  `json:"file_sizes"`
}

// backupFiles lists the data files to include in a backup.
// backupRelFiles lists relative file paths (under DataDir) to include in a backup.
var backupRelFiles = []string{
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

// backupRelDirs lists relative directory paths (under DataDir) to recurse.
var backupRelDirs = []string{
	"sessions",
	"persona/skills",
	"plugins",
}

func backupFiles() []string {
	out := make([]string, len(backupRelFiles))
	for i, f := range backupRelFiles {
		out[i] = filepath.Join(appdir.DataDir(), f)
	}
	return out
}

func backupDirs() []string {
	out := make([]string, len(backupRelDirs))
	for i, d := range backupRelDirs {
		out[i] = filepath.Join(appdir.DataDir(), d)
	}
	return out
}

// handleBackupExport creates a ZIP backup and streams it to the client.
func (g *Gateway) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("yunque-backup-%s.zip", stamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	zw := zip.NewWriter(w)
	defer zw.Close()

	manifest := BackupManifest{
		AgentVersion:    version.Version,
		ExportedAt:      time.Now().UTC(),
		BackupType:      "manual",
		ManifestVersion: "1.0",
		Files:           make(map[string]string),
		FileSizes:       make(map[string]int64),
	}

	dataRoot := appdir.DataDir()

	// Add individual files
	for _, abs := range backupFiles() {
		rel, _ := filepath.Rel(dataRoot, abs)
		if err := addFileToZip(zw, abs, filepath.ToSlash(rel), &manifest); err != nil {
			slog.Debug("backup: skip file", "path", abs, "err", err)
		}
	}

	// Add directory contents
	for _, dir := range backupDirs() {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(dataRoot, path)
				return addFileToZip(zw, path, filepath.ToSlash(rel), &manifest)
			})
		}
	}

	// Write manifest
	mdata, _ := json.MarshalIndent(manifest, "", "  ")
	mw, _ := zw.Create("manifest.json")
	mw.Write(mdata)

	slog.Info("backup exported", "files", len(manifest.Files), "filename", filename)
}

// handleBackupImport restores from an uploaded ZIP backup.
func (g *Gateway) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit upload to 500MB
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)

	file, _, err := r.FormFile("backup")
	if err != nil {
		http.Error(w, "missing backup file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Write to temporary file for zip.OpenReader
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

	// Read and validate manifest
	var manifest *BackupManifest
	for _, f := range zr.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				break
			}
			var m BackupManifest
			json.NewDecoder(rc).Decode(&m)
			rc.Close()
			manifest = &m
			break
		}
	}

	if manifest == nil {
		http.Error(w, "backup archive missing manifest.json", http.StatusBadRequest)
		return
	}

	// Version compatibility check
	versionWarning := ""
	if manifest.AgentVersion != version.Version {
		backupMajor := majorVersion(manifest.AgentVersion)
		currentMajor := majorVersion(version.Version)
		if backupMajor != currentMajor {
			apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("major version mismatch: backup=%s current=%s", manifest.AgentVersion, version.Version))
			return
		}
		versionWarning = fmt.Sprintf("minor version difference: backup=%s current=%s", manifest.AgentVersion, version.Version)
	}

	dataRoot := appdir.DataDir()

	// Validate all file checksums before writing anything back to disk.
	for _, f := range zr.File {
		if f.Name == "manifest.json" || f.FileInfo().IsDir() {
			continue
		}

		cleanName := filepath.Clean(filepath.FromSlash(f.Name))
		if strings.Contains(cleanName, "..") {
			apperror.WriteCode(w, apperror.CodeBadRequest, "backup archive contains invalid traversal path")
			return
		}

		expected, ok := manifest.Files[f.Name]
		if !ok || expected == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("unexpected file in backup: %s", f.Name))
			return
		}

		rc, err := f.Open()
		if err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "failed to read backup entry: "+f.Name)
			return
		}
		h := sha256.New()
		if _, err := io.Copy(h, rc); err != nil {
			rc.Close()
			apperror.WriteCode(w, apperror.CodeBadRequest, "failed to validate backup entry: "+f.Name)
			return
		}
		rc.Close()

		actualHash := hex.EncodeToString(h.Sum(nil))
		if actualHash != expected {
			apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("checksum mismatch for %s", f.Name))
			return
		}
	}

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

		targetPath := filepath.Join(dataRoot, cleanName)

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

// handleBackupInfo returns information about what would be backed up.
func (g *Gateway) handleBackupInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files := make(map[string]int64)
	dataRoot := appdir.DataDir()

	for _, abs := range backupFiles() {
		if info, err := os.Stat(abs); err == nil {
			rel, _ := filepath.Rel(dataRoot, abs)
			files[filepath.ToSlash(rel)] = info.Size()
		}
	}

	for _, dir := range backupDirs() {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(dataRoot, path)
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
		"version":     version.Version,
	})
}

// addFileToZip adds a single file to the zip writer and updates the manifest.
func addFileToZip(zw *zip.Writer, absPath, archiveName string, manifest *BackupManifest) error {
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

// majorVersion extracts the major.minor version prefix (e.g. "1.2" from "1.2.3").
func majorVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}
