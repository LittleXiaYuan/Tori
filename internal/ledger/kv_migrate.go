package ledger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"ledger"
)

// KVMigrator migrates legacy JSON files into Ledger's KV store.
// Once migrated, the JSON file is renamed to .migrated to prevent re-migration.
//
// Usage during bootstrap:
//
//	migrator := NewKVMigrator(ldg)
//	migrator.MigrateFile("trust", "scores", cfg.DataPath("trust_scores.json"))
//	migrator.MigrateFile("emotion", "history", cfg.DataPath("emotion_history.json"))
type KVMigrator struct {
	ldg *ledger.Ledger
}

func NewKVMigrator(ldg *ledger.Ledger) *KVMigrator {
	return &KVMigrator{ldg: ldg}
}

// MigrateFile reads a JSON file and stores it as a KV entry.
// If the file doesn't exist, this is a no-op.
// After successful migration, the file is renamed to .migrated.
func (m *KVMigrator) MigrateFile(namespace, key, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("kv migrate read %s: %w", filePath, err)
	}

	data = []byte(strings.TrimPrefix(string(data), "\xef\xbb\xbf"))
	if len(data) == 0 {
		return nil
	}

	ctx := context.Background()
	if err := m.ldg.KV.PutRaw(ctx, namespace, key, data); err != nil {
		return fmt.Errorf("kv migrate put %s/%s: %w", namespace, key, err)
	}

	migratedPath := filePath + ".migrated"
	if err := os.Rename(filePath, migratedPath); err != nil {
		slog.Warn("kv migrate: rename failed (data already in Ledger)", "file", filePath, "err", err)
	} else {
		slog.Info("kv migrated", "file", filepath.Base(filePath), "namespace", namespace, "key", key)
	}
	return nil
}

// MigrateDir migrates all JSON files in a directory into a namespace,
// using the filename (without extension) as the key.
func (m *KVMigrator) MigrateDir(namespace, dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	migrated := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(dirPath, entry.Name())
		if err := m.MigrateFile(namespace, key, filePath); err != nil {
			slog.Warn("kv migrate dir: skip file", "file", entry.Name(), "err", err)
			continue
		}
		migrated++
	}
	return migrated, nil
}

// KVConfigStore wraps Ledger KV for a specific namespace, providing
// simple Get/Put methods that replace JSON file read/write patterns.
type KVConfigStore struct {
	ldg       *ledger.Ledger
	namespace string
}

func NewKVConfigStore(ldg *ledger.Ledger, namespace string) *KVConfigStore {
	return &KVConfigStore{ldg: ldg, namespace: namespace}
}

func (s *KVConfigStore) Put(ctx context.Context, key string, value any) error {
	return s.ldg.KV.Put(ctx, s.namespace, key, value)
}

func (s *KVConfigStore) Get(ctx context.Context, key string, dest any) (bool, error) {
	return s.ldg.KV.Get(ctx, s.namespace, key, dest)
}

func (s *KVConfigStore) Delete(ctx context.Context, key string) error {
	return s.ldg.KV.Delete(ctx, s.namespace, key)
}

func (s *KVConfigStore) PutRaw(ctx context.Context, key string, data []byte) error {
	return s.ldg.KV.PutRaw(ctx, s.namespace, key, data)
}

func (s *KVConfigStore) GetRaw(ctx context.Context, key string) ([]byte, error) {
	entry, err := s.ldg.KV.GetRaw(ctx, s.namespace, key)
	if err != nil {
		return nil, err
	}
	return entry.Value, nil
}
