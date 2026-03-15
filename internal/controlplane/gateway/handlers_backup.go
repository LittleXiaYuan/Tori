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
var backupFiles = []string{
	"data/memory.json",
	"data/adaptive.json",
	"data/graph.json",
	"data/editable.json",
	"data/audit.jsonl",
	"data/mcp.json",
	"data/cron/jobs.json",
	"data/persona/IDENTITY.md",
	"data/persona/SOUL.md",
}

// backupDirs lists directories whose contents should be recursively included.
var backupDirs = []string{
	"data/sessions",
	"data/persona/skills",
	"data/plugins",
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

	// Add individual files
	for _, rel := range backupFiles {
		if err := addFileToZip(zw, rel, &manifest); err != nil {
			slog.Debug("backup: skip file", "path", rel, "err", err)
		}
	}

	// Add directory contents
	for _, dir := range backupDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel := filepath.ToSlash(path)
				return addFileToZip(zw, rel, &manifest)
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
			http.Error(w, fmt.Sprintf("major version mismatch: backup=%s current=%s", manifest.AgentVersion, version.Version), http.StatusConflict)
			return
		}
		versionWarning = fmt.Sprintf("minor version difference: backup=%s current=%s", manifest.AgentVersion, version.Version)
	}

	// Extract files (only to known safe paths under data/)
	restored := 0
	for _, f := range zr.File {
		if f.Name == "manifest.json" {
			continue
		}

		// Security: only allow extraction under data/
		cleanName := filepath.Clean(filepath.FromSlash(f.Name))
		if !strings.HasPrefix(cleanName, "data"+string(filepath.Separator)) && !strings.HasPrefix(cleanName, "data\\") && cleanName != "data" {
			slog.Warn("backup: skip unsafe path", "path", f.Name)
			continue
		}

		// Prevent path traversal
		if strings.Contains(cleanName, "..") {
			slog.Warn("backup: skip traversal path", "path", f.Name)
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(cleanName, 0o755)
			continue
		}

		// Verify checksum if available
		rc, err := f.Open()
		if err != nil {
			continue
		}

		dir := filepath.Dir(cleanName)
		os.MkdirAll(dir, 0o755)

		outFile, err := os.Create(cleanName)
		if err != nil {
			rc.Close()
			continue
		}

		h := sha256.New()
		mw := io.MultiWriter(outFile, h)
		io.Copy(mw, rc)
		rc.Close()
		outFile.Close()

		actualHash := hex.EncodeToString(h.Sum(nil))
		if expected, ok := manifest.Files[f.Name]; ok && expected != actualHash {
			slog.Warn("backup: checksum mismatch", "file", f.Name, "expected", expected, "actual", actualHash)
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

	for _, rel := range backupFiles {
		if info, err := os.Stat(rel); err == nil {
			files[rel] = info.Size()
		}
	}

	for _, dir := range backupDirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			files[filepath.ToSlash(path)] = info.Size()
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
func addFileToZip(zw *zip.Writer, rel string, manifest *BackupManifest) error {
	data, err := os.ReadFile(rel)
	if err != nil {
		return err
	}

	h := sha256.Sum256(data)
	manifest.Files[rel] = hex.EncodeToString(h[:])
	manifest.FileSizes[rel] = int64(len(data))

	w, err := zw.Create(rel)
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
