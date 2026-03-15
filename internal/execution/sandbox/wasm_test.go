package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWasmSandboxNew(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())
	if ws == nil {
		t.Fatal("expected non-nil WasmSandbox")
	}
	stats := ws.Stats()
	if stats["memory_limit_pages"].(uint32) != 256 {
		t.Errorf("expected 256 pages, got %v", stats["memory_limit_pages"])
	}
	if stats["wasm_runtime"] != nil {
		// Stats doesn't include runtime info, that's in SystemInfo
	}
}

func TestWasmSandboxKVStore(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())

	ws.SetKV("key1", "value1")
	ws.SetKV("key2", "value2")

	v, ok := ws.GetKV("key1")
	if !ok || v != "value1" {
		t.Errorf("expected value1, got %s", v)
	}

	_, ok = ws.GetKV("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent key")
	}

	ws.ClearKV()
	_, ok = ws.GetKV("key1")
	if ok {
		t.Error("expected cleared KV store")
	}
}

func TestWasmSandboxEmptyModule(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())
	_, err := ws.Execute(context.Background(), nil, "", "")
	if err == nil {
		t.Error("expected error for empty module")
	}
}

func TestWasmSandboxInvalidModule(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())
	result, err := ws.Execute(context.Background(), []byte("not wasm"), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != -1 {
		t.Errorf("expected exit code -1, got %d", result.ExitCode)
	}
	if result.Stderr == "" {
		t.Error("expected compile error in stderr")
	}
}

// Minimal valid WASM module (empty, exports nothing useful)
// This is the smallest valid wasm binary: magic + version + no sections
var minimalWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic: \0asm
	0x01, 0x00, 0x00, 0x00, // version: 1
}

func TestWasmSandboxNoEntryPoint(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())
	result, err := ws.Execute(context.Background(), minimalWasm, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != -1 {
		t.Errorf("expected exit code -1 for no entry point, got %d", result.ExitCode)
	}
}

func TestWasmSandboxTimeout(t *testing.T) {
	ws := NewWasmSandbox(WasmConfig{
		MemoryLimitPages: 16,
		MaxDuration:      50 * time.Millisecond,
		MaxOutputBytes:   1024,
	})
	// Minimal module won't timeout but config is set correctly
	stats := ws.Stats()
	if stats["max_duration"].(string) != "50ms" {
		t.Errorf("expected 50ms, got %v", stats["max_duration"])
	}
}

func TestWasmSandboxRegisterHostFunc(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())
	ws.RegisterHostFunc("my_func", func(ctx context.Context, args []uint64) ([]uint64, error) {
		return []uint64{42}, nil
	})
	stats := ws.Stats()
	if stats["host_funcs"].(int) != 1 {
		t.Errorf("expected 1 host func, got %v", stats["host_funcs"])
	}
}

func TestSystemInfoIncludesWasm(t *testing.T) {
	info := SystemInfo()
	types := info["sandbox_types"]
	if types == "" {
		t.Fatal("expected sandbox_types")
	}
	if !strings.Contains(types, "wasm") {
		t.Errorf("expected 'wasm' in sandbox_types, got %s", types)
	}
	if info["wasm_runtime"] != "wazero" {
		t.Errorf("expected wazero runtime, got %v", info["wasm_runtime"])
	}
}

func TestDefaultWasmConfig(t *testing.T) {
	cfg := DefaultWasmConfig()
	if cfg.MemoryLimitPages != 256 {
		t.Errorf("expected 256 pages, got %d", cfg.MemoryLimitPages)
	}
	if cfg.MaxDuration != 10*time.Second {
		t.Errorf("expected 10s, got %v", cfg.MaxDuration)
	}
	if cfg.MaxOutputBytes != 64*1024 {
		t.Errorf("expected 64KB, got %d", cfg.MaxOutputBytes)
	}
}
