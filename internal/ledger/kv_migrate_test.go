package ledger

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LittleXiaYuan/ledger"
	lsqlite "github.com/LittleXiaYuan/ledger/backend/sqlite"
)

func newTestLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	backend, err := lsqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	ldg, err := ledger.Open(backend)
	if err != nil {
		backend.Close()
		t.Fatalf("open ledger: %v", err)
	}
	t.Cleanup(func() { ldg.Close() })
	return ldg
}

func TestKVConfigStore_PutGet(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewKVConfigStore(ldg, "test_ns")

	ctx := context.Background()

	type Config struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	err := store.Put(ctx, "cfg1", Config{Name: "alpha", Value: 42})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	var got Config
	found, err := store.Get(ctx, "cfg1", &got)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected to find cfg1")
	}
	if got.Name != "alpha" || got.Value != 42 {
		t.Errorf("got %+v, want {alpha 42}", got)
	}
}

func TestKVConfigStore_GetMissing(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewKVConfigStore(ldg, "test_ns")

	var dest map[string]string
	found, err := store.Get(context.Background(), "nonexistent", &dest)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Error("expected not found for missing key")
	}
}

func TestKVConfigStore_Delete(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewKVConfigStore(ldg, "test_ns")
	ctx := context.Background()

	store.Put(ctx, "to_delete", "value")
	err := store.Delete(ctx, "to_delete")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var dest string
	found, _ := store.Get(ctx, "to_delete", &dest)
	if found {
		t.Error("expected key to be deleted")
	}
}

func TestKVConfigStore_PutRawGetRaw(t *testing.T) {
	ldg := newTestLedger(t)
	store := NewKVConfigStore(ldg, "raw_ns")
	ctx := context.Background()

	raw := []byte(`{"key":"raw_value"}`)
	err := store.PutRaw(ctx, "raw1", raw)
	if err != nil {
		t.Fatalf("PutRaw: %v", err)
	}

	got, err := store.GetRaw(ctx, "raw1")
	if err != nil {
		t.Fatalf("GetRaw: %v", err)
	}
	if string(got) != string(raw) {
		t.Errorf("GetRaw = %s, want %s", got, raw)
	}
}

func TestKVConfigStore_NamespaceIsolation(t *testing.T) {
	ldg := newTestLedger(t)
	storeA := NewKVConfigStore(ldg, "ns_a")
	storeB := NewKVConfigStore(ldg, "ns_b")
	ctx := context.Background()

	storeA.Put(ctx, "shared_key", "from_a")
	storeB.Put(ctx, "shared_key", "from_b")

	var gotA, gotB string
	storeA.Get(ctx, "shared_key", &gotA)
	storeB.Get(ctx, "shared_key", &gotB)

	if gotA != "from_a" {
		t.Errorf("ns_a got %s, want from_a", gotA)
	}
	if gotB != "from_b" {
		t.Errorf("ns_b got %s, want from_b", gotB)
	}
}

func TestKVMigrator_MigrateFile(t *testing.T) {
	ldg := newTestLedger(t)
	migrator := NewKVMigrator(ldg)

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "test_config.json")
	os.WriteFile(jsonPath, []byte(`{"setting":"value"}`), 0644)

	err := migrator.MigrateFile("config", "main", jsonPath)
	if err != nil {
		t.Fatalf("MigrateFile: %v", err)
	}

	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("original file should be renamed")
	}
	if _, err := os.Stat(jsonPath + ".migrated"); err != nil {
		t.Errorf("migrated file should exist: %v", err)
	}

	store := NewKVConfigStore(ldg, "config")
	raw, err := store.GetRaw(context.Background(), "main")
	if err != nil {
		t.Fatalf("GetRaw after migrate: %v", err)
	}
	if string(raw) != `{"setting":"value"}` {
		t.Errorf("migrated data = %s", raw)
	}
}

func TestKVMigrator_MigrateFile_NotExist(t *testing.T) {
	ldg := newTestLedger(t)
	migrator := NewKVMigrator(ldg)

	err := migrator.MigrateFile("ns", "key", "/nonexistent/path.json")
	if err != nil {
		t.Fatalf("should be no-op for missing file, got: %v", err)
	}
}

func TestKVMigrator_MigrateFile_Empty(t *testing.T) {
	ldg := newTestLedger(t)
	migrator := NewKVMigrator(ldg)

	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.json")
	os.WriteFile(emptyPath, []byte{}, 0644)

	err := migrator.MigrateFile("ns", "key", emptyPath)
	if err != nil {
		t.Fatalf("should skip empty file, got: %v", err)
	}
}

func TestKVMigrator_MigrateFile_BOM(t *testing.T) {
	ldg := newTestLedger(t)
	migrator := NewKVMigrator(ldg)

	dir := t.TempDir()
	bomPath := filepath.Join(dir, "bom.json")
	os.WriteFile(bomPath, append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"bom":"stripped"}`)...), 0644)

	err := migrator.MigrateFile("bom_ns", "key", bomPath)
	if err != nil {
		t.Fatalf("MigrateFile with BOM: %v", err)
	}

	store := NewKVConfigStore(ldg, "bom_ns")
	raw, _ := store.GetRaw(context.Background(), "key")
	if string(raw) != `{"bom":"stripped"}` {
		t.Errorf("BOM not stripped, got: %s", raw)
	}
}

func TestKVMigrator_MigrateDir(t *testing.T) {
	ldg := newTestLedger(t)
	migrator := NewKVMigrator(ldg)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(`"val_a"`), 0644)
	os.WriteFile(filepath.Join(dir, "b.json"), []byte(`"val_b"`), 0644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte(`skip`), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	count, err := migrator.MigrateDir("dir_ns", dir)
	if err != nil {
		t.Fatalf("MigrateDir: %v", err)
	}
	if count != 2 {
		t.Errorf("migrated %d files, want 2", count)
	}

	store := NewKVConfigStore(ldg, "dir_ns")
	rawA, _ := store.GetRaw(context.Background(), "a")
	if string(rawA) != `"val_a"` {
		t.Errorf("a = %s", rawA)
	}
}

func TestKVMigrator_MigrateDir_NotExist(t *testing.T) {
	ldg := newTestLedger(t)
	migrator := NewKVMigrator(ldg)

	count, err := migrator.MigrateDir("ns", "/nonexistent/dir")
	if err != nil {
		t.Fatalf("should be no-op, got: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}
