package appdir

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// MaybeMigrateLegacy checks if a legacy data/ directory exists next to the
// executable and, if so, copies its contents to the new DataDir location.
//
// The migration is skipped when:
//   - The new DataDir already contains data (non-empty)
//   - The legacy directory doesn't exist
//   - DataDir resolves to the same path as the legacy directory
//
// After a successful migration the legacy directory is renamed to
// data.migrated-YYYYMMDD as a backup.
func MaybeMigrateLegacy() error {
	newDir := DataDir()
	legacyDir := LegacyDataDir()

	if filepath.Clean(newDir) == filepath.Clean(legacyDir) {
		return nil
	}

	legacyInfo, err := os.Stat(legacyDir)
	if err != nil || !legacyInfo.IsDir() {
		return nil
	}

	entries, _ := os.ReadDir(legacyDir)
	if len(entries) == 0 {
		return nil
	}

	newEntries, _ := os.ReadDir(newDir)
	if len(newEntries) > 0 {
		slog.Info("appdir: skipping migration, target already has data",
			"legacy", legacyDir, "new", newDir)
		return nil
	}

	slog.Info("appdir: migrating legacy data directory",
		"from", legacyDir, "to", newDir)

	if err := copyDir(legacyDir, newDir); err != nil {
		return fmt.Errorf("appdir: migration copy failed: %w", err)
	}

	backupName := legacyDir + ".migrated-" + time.Now().Format("20060102")
	if err := os.Rename(legacyDir, backupName); err != nil {
		slog.Warn("appdir: could not rename legacy dir after migration",
			"err", err, "legacy", legacyDir)
	} else {
		slog.Info("appdir: legacy dir backed up", "backup", backupName)
	}

	slog.Info("appdir: migration complete", "new_data_dir", newDir)
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
