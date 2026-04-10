package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"yunque-agent/internal/appdir"
	"yunque-agent/internal/version"
)

// Config for the auto-backup scheduler.
type Config struct {
	Enabled       bool
	BackupDir     string        // where to write zip files; default: DataDir/backups
	Interval      time.Duration // how often; default: 24h
	MaxBackups    int           // keep at most N; default: 7
	ScheduleHour  int           // hour of day (0-23) to run; default: 4
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		BackupDir:    filepath.Join(appdir.DataDir(), "backups"),
		Interval:     24 * time.Hour,
		MaxBackups:   7,
		ScheduleHour: 4,
	}
}

// relFiles and relDirs define what to back up (relative to DataDir).
var relFiles = []string{
	"memory.json", "adaptive.json", "graph.json", "editable.json",
	"audit.jsonl", "mcp.json", "cron/jobs.json",
	"persona/IDENTITY.md", "persona/SOUL.md",
	"providers.json", "market.json",
}

var relDirs = []string{
	"sessions", "persona/skills", "plugins", "ledger",
}

type manifest struct {
	AgentVersion string            `json:"agent_version"`
	ExportedAt   time.Time         `json:"exported_at"`
	BackupType   string            `json:"backup_type"`
	Files        map[string]string `json:"files"`
}

// StartScheduler runs auto-backups in the background until ctx is cancelled.
func StartScheduler(ctx context.Context, cfg Config) {
	if !cfg.Enabled {
		slog.Info("auto-backup: disabled")
		return
	}
	os.MkdirAll(cfg.BackupDir, 0755)
	slog.Info("auto-backup: scheduled", "dir", cfg.BackupDir, "hour", cfg.ScheduleHour, "max", cfg.MaxBackups)

	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), cfg.ScheduleHour, 0, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			if err := RunBackup(cfg); err != nil {
				slog.Error("auto-backup: failed", "err", err)
			}
		}
	}
}

// RunBackup creates a backup zip and prunes old backups.
func RunBackup(cfg Config) error {
	os.MkdirAll(cfg.BackupDir, 0755)

	stamp := time.Now().Format("20060102-150405")
	outPath := filepath.Join(cfg.BackupDir, fmt.Sprintf("yunque-auto-%s.zip", stamp))

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	m := manifest{
		AgentVersion: version.Version,
		ExportedAt:   time.Now().UTC(),
		BackupType:   "auto",
		Files:        make(map[string]string),
	}

	dataRoot := appdir.DataDir()

	for _, rel := range relFiles {
		abs := filepath.Join(dataRoot, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		h := sha256.Sum256(data)
		m.Files[rel] = hex.EncodeToString(h[:])
		w, err := zw.Create(rel)
		if err != nil {
			continue
		}
		w.Write(data)
	}

	for _, rel := range relDirs {
		dirAbs := filepath.Join(dataRoot, rel)
		filepath.WalkDir(dirAbs, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(dataRoot, path)
			archiveName := filepath.ToSlash(relPath)
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			h := sha256.Sum256(data)
			m.Files[archiveName] = hex.EncodeToString(h[:])
			w, err := zw.Create(archiveName)
			if err != nil {
				return nil
			}
			w.Write(data)
			return nil
		})
	}

	mdata, _ := json.MarshalIndent(m, "", "  ")
	mw, _ := zw.Create("manifest.json")
	mw.Write(mdata)

	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}

	info, _ := os.Stat(outPath)
	slog.Info("auto-backup: created", "path", outPath, "files", len(m.Files), "size_bytes", info.Size())

	pruneOldBackups(cfg.BackupDir, cfg.MaxBackups)
	return nil
}

func pruneOldBackups(dir string, maxKeep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "yunque-auto-") && strings.HasSuffix(e.Name(), ".zip") {
			backups = append(backups, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(backups)
	for len(backups) > maxKeep {
		oldest := backups[0]
		backups = backups[1:]
		if err := os.Remove(oldest); err == nil {
			slog.Info("auto-backup: pruned old backup", "path", oldest)
		}
	}
}
