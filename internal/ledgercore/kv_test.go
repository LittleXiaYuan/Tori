package ledger_test

import (
	"context"
	"errors"
	"testing"

	"yunque-agent/internal/ledgercore"
	lsqlite "yunque-agent/internal/ledgercore/backend/sqlite"
)

func openTestLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	backend, err := lsqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ldg, err := ledger.Open(backend)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ldg.Close() })
	return ldg
}

func TestKVPutGet(t *testing.T) {
	ldg := openTestLedger(t)
	ctx := context.Background()

	type Config struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	if err := ldg.KV.Put(ctx, "test", "key1", Config{Name: "hello", Value: 42}); err != nil {
		t.Fatal(err)
	}

	var got Config
	found, err := ldg.KV.Get(ctx, "test", "key1", &got)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected key to exist")
	}
	if got.Name != "hello" || got.Value != 42 {
		t.Errorf("got %+v, want {hello 42}", got)
	}
}

func TestKVGetMissing(t *testing.T) {
	ldg := openTestLedger(t)
	ctx := context.Background()

	var got string
	found, err := ldg.KV.Get(ctx, "ns", "nonexistent", &got)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected not found for missing key")
	}
}

func TestKVGetPropagatesBackendError(t *testing.T) {
	backend, err := lsqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ldg, err := ledger.Open(backend)
	if err != nil {
		t.Fatal(err)
	}
	if err := ldg.Close(); err != nil {
		t.Fatal(err)
	}

	var got string
	found, err := ldg.KV.Get(context.Background(), "ns", "k", &got)
	if err == nil {
		t.Fatal("expected backend error after close")
	}
	if found {
		t.Fatal("closed backend should not look like a found key")
	}
	if errors.Is(err, ledger.ErrKVNotFound) {
		t.Fatalf("closed backend must not be reported as missing key: %v", err)
	}
}

func TestKVUpsert(t *testing.T) {
	ldg := openTestLedger(t)
	ctx := context.Background()

	if err := ldg.KV.Put(ctx, "ns", "k", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := ldg.KV.Put(ctx, "ns", "k", "v2"); err != nil {
		t.Fatal(err)
	}

	var got string
	found, err := ldg.KV.Get(ctx, "ns", "k", &got)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got != "v2" {
		t.Errorf("upsert failed: got %q, want v2", got)
	}
}

func TestKVDelete(t *testing.T) {
	ldg := openTestLedger(t)
	ctx := context.Background()

	ldg.KV.Put(ctx, "ns", "k", "v")
	if err := ldg.KV.Delete(ctx, "ns", "k"); err != nil {
		t.Fatal(err)
	}

	var got string
	found, _ := ldg.KV.Get(ctx, "ns", "k", &got)
	if found {
		t.Error("expected key to be deleted")
	}
}

func TestKVList(t *testing.T) {
	ldg := openTestLedger(t)
	ctx := context.Background()

	ldg.KV.Put(ctx, "ns1", "a", "1")
	ldg.KV.Put(ctx, "ns1", "b", "2")
	ldg.KV.Put(ctx, "ns2", "c", "3")

	entries, err := ldg.KV.List(ctx, "ns1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("list ns1: got %d entries, want 2", len(entries))
	}

	keys, err := ldg.KV.ListKeys(ctx, "ns1")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("listkeys: got %v, want [a b]", keys)
	}
}

func TestKVNamespaceIsolation(t *testing.T) {
	ldg := openTestLedger(t)
	ctx := context.Background()

	ldg.KV.Put(ctx, "trust", "scores", "trust_data")
	ldg.KV.Put(ctx, "emotion", "scores", "emotion_data")

	var got string
	ldg.KV.Get(ctx, "trust", "scores", &got)
	if got != "trust_data" {
		t.Errorf("namespace isolation failed: got %q", got)
	}

	ldg.KV.Get(ctx, "emotion", "scores", &got)
	if got != "emotion_data" {
		t.Errorf("namespace isolation failed: got %q", got)
	}
}

func TestKVPutRaw(t *testing.T) {
	ldg := openTestLedger(t)
	ctx := context.Background()

	raw := []byte(`{"custom":"json"}`)
	if err := ldg.KV.PutRaw(ctx, "raw", "data", raw); err != nil {
		t.Fatal(err)
	}

	entry, err := ldg.KV.GetRaw(ctx, "raw", "data")
	if err != nil {
		t.Fatal(err)
	}
	if string(entry.Value) != `{"custom":"json"}` {
		t.Errorf("raw roundtrip failed: got %s", entry.Value)
	}
}
