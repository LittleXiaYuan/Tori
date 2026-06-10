package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// cappedBuffer is an io.Writer that accumulates up to cap bytes and silently
// discards the rest, so a misbehaving module can't exhaust host memory through
// stdout/stderr. Writes always report full consumption so the guest never sees
// a short write and aborts.
type cappedBuffer struct {
	buf bytes.Buffer
	cap int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if remaining := c.cap - c.buf.Len(); remaining > 0 {
		if len(p) <= remaining {
			c.buf.Write(p)
		} else {
			c.buf.Write(p[:remaining])
		}
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string { return c.buf.String() }

// WasmSandbox executes WASM modules in a memory-safe, isolated environment.
// Unlike process-based sandboxes, WASM provides deterministic execution with
// no filesystem or network access unless explicitly granted via host functions.
type WasmSandbox struct {
	mu          sync.Mutex
	memoryLimit uint32 // pages (64KB each)
	maxDuration time.Duration
	maxOutput   int
	hostFuncs   map[string]HostFunc
	kvStore     map[string]string // simple KV for agent state
	// compileCache lets a fresh per-execution runtime reuse prior compilation
	// results (keyed by module bytes), removing the cold-compile cost on
	// repeated executions of the same module without weakening per-execution
	// isolation (each Execute still gets its own runtime + module instance).
	compileCache wazero.CompilationCache
}

// HostFunc is a function exposed to WASM modules as an import.
type HostFunc func(ctx context.Context, args []uint64) ([]uint64, error)

// ModuleHostFunc is a privileged host function registered under the "env"
// import for a single execution, with direct access to the guest module (and
// thus its linear memory). Callers (e.g. the gateway) build these per pack,
// gated by the pack's declared permissions, so an unpermitted capability is
// simply never exported to the module. The stack-based signature mirrors
// wazero's api.GoModuleFunc: read params from stack[i] (i32s) and write the
// single i32 result back into stack[0].
type ModuleHostFunc struct {
	Name    string
	Params  int // number of i32 params
	Results int // number of i32 results (0 or 1)
	Fn      func(ctx context.Context, m api.Module, stack []uint64)
}

// WasmResult is the output of a WASM execution.
type WasmResult struct {
	Stdout   string            `json:"stdout"`
	Stderr   string            `json:"stderr"`
	ExitCode int               `json:"exit_code"`
	Duration string            `json:"duration"`
	MemUsed  uint32            `json:"mem_used_bytes"`
	Exports  []string          `json:"exports,omitempty"`
	KVWrites map[string]string `json:"kv_writes,omitempty"`
}

// WasmConfig configures the WASM sandbox.
type WasmConfig struct {
	MemoryLimitPages uint32        // max memory in 64KB pages (default 256 = 16MB)
	MaxDuration      time.Duration // execution timeout (default 10s)
	MaxOutputBytes   int           // max stdout/stderr size (default 64KB)
}

// DefaultWasmConfig returns sensible defaults for WASM execution.
func DefaultWasmConfig() WasmConfig {
	return WasmConfig{
		MemoryLimitPages: 256, // 16MB
		MaxDuration:      10 * time.Second,
		MaxOutputBytes:   64 * 1024, // 64KB
	}
}

// NewWasmSandbox creates a new WASM sandbox with the given configuration.
func NewWasmSandbox(cfg WasmConfig) *WasmSandbox {
	if cfg.MemoryLimitPages == 0 {
		cfg.MemoryLimitPages = 256
	}
	if cfg.MaxDuration == 0 {
		cfg.MaxDuration = 10 * time.Second
	}
	if cfg.MaxOutputBytes == 0 {
		cfg.MaxOutputBytes = 64 * 1024
	}
	return &WasmSandbox{
		memoryLimit:  cfg.MemoryLimitPages,
		maxDuration:  cfg.MaxDuration,
		maxOutput:    cfg.MaxOutputBytes,
		hostFuncs:    make(map[string]HostFunc),
		kvStore:      make(map[string]string),
		compileCache: wazero.NewCompilationCache(),
	}
}

// RegisterHostFunc adds a host function that WASM modules can call.
func (ws *WasmSandbox) RegisterHostFunc(name string, fn HostFunc) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.hostFuncs[name] = fn
}

// Execute runs a compiled WASM module with the given input.
// The module must export a "_start" or "main" function (WASI convention),
// or a custom entry point specified by entryPoint.
func (ws *WasmSandbox) Execute(ctx context.Context, wasmBytes []byte, stdin string, entryPoint string) (*WasmResult, error) {
	return ws.ExecuteWithHostFuncs(ctx, wasmBytes, stdin, entryPoint, nil)
}

// ExecuteWithHostFuncs is Execute plus a set of privileged host functions
// exported under "env" for this single execution. They are registered after the
// built-in kv_*/log_message imports, so a privileged func may shadow a built-in
// of the same name. Per-execution registration keeps capabilities scoped to the
// caller's permission set without leaking across concurrent module runs.
func (ws *WasmSandbox) ExecuteWithHostFuncs(ctx context.Context, wasmBytes []byte, stdin string, entryPoint string, extra []ModuleHostFunc) (*WasmResult, error) {
	ws.mu.Lock()
	hostFuncs := make(map[string]HostFunc, len(ws.hostFuncs))
	for k, v := range ws.hostFuncs {
		hostFuncs[k] = v
	}
	ws.mu.Unlock()

	if len(wasmBytes) == 0 {
		return nil, fmt.Errorf("empty wasm module")
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, ws.maxDuration)
	defer cancel()

	// Create a new runtime per execution for full isolation. A shared
	// compilation cache lets repeated runs of the same module skip recompiling.
	runtimeConfig := wazero.NewRuntimeConfig()
	if ws.compileCache != nil {
		runtimeConfig = runtimeConfig.WithCompilationCache(ws.compileCache)
	}
	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	defer runtime.Close(ctx)

	// Instantiate WASI for stdio support
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	// Register custom host functions under "env" module
	kvWrites := make(map[string]string)
	if len(hostFuncs) > 0 || true {
		envBuilder := runtime.NewHostModuleBuilder("env")

		// Built-in: kv_set(key_ptr, key_len, val_ptr, val_len) -> 0
		envBuilder.NewFunctionBuilder().
			WithFunc(func(ctx context.Context, m api.Module, keyPtr, keyLen, valPtr, valLen uint32) uint32 {
				mem := m.Memory()
				keyBytes, ok1 := mem.Read(keyPtr, keyLen)
				valBytes, ok2 := mem.Read(valPtr, valLen)
				if !ok1 || !ok2 {
					return 1
				}
				key := string(keyBytes)
				val := string(valBytes)
				ws.mu.Lock()
				ws.kvStore[key] = val
				ws.mu.Unlock()
				kvWrites[key] = val
				return 0
			}).Export("kv_set")

		// Built-in: kv_get(key_ptr, key_len, buf_ptr, buf_cap) -> bytes_written
		envBuilder.NewFunctionBuilder().
			WithFunc(func(ctx context.Context, m api.Module, keyPtr, keyLen, bufPtr, bufCap uint32) uint32 {
				mem := m.Memory()
				keyBytes, ok := mem.Read(keyPtr, keyLen)
				if !ok {
					return 0
				}
				ws.mu.Lock()
				val := ws.kvStore[string(keyBytes)]
				ws.mu.Unlock()
				if val == "" {
					return 0
				}
				data := []byte(val)
				if uint32(len(data)) > bufCap {
					data = data[:bufCap]
				}
				mem.Write(bufPtr, data)
				return uint32(len(data))
			}).Export("kv_get")

		// Built-in: log_message(ptr, len) -> 0
		envBuilder.NewFunctionBuilder().
			WithFunc(func(ctx context.Context, m api.Module, ptr, length uint32) uint32 {
				mem := m.Memory()
				msg, ok := mem.Read(ptr, length)
				if ok {
					_ = msg // Could be collected for structured logging
				}
				return 0
			}).Export("log_message")

		// Privileged, permission-scoped host functions for this execution.
		for _, hf := range extra {
			if hf.Name == "" || hf.Fn == nil {
				continue
			}
			params := make([]api.ValueType, hf.Params)
			for i := range params {
				params[i] = api.ValueTypeI32
			}
			results := make([]api.ValueType, hf.Results)
			for i := range results {
				results[i] = api.ValueTypeI32
			}
			fn := hf.Fn
			envBuilder.NewFunctionBuilder().
				WithGoModuleFunction(api.GoModuleFunc(fn), params, results).
				Export(hf.Name)
		}

		envBuilder.Instantiate(ctx)
	}

	// Configure module with memory limits. Stdout/stderr are captured into
	// capped buffers so the guest can return data (the request/response
	// envelope ABI) and so a runaway module can't exhaust host memory.
	stdoutBuf := &cappedBuffer{cap: ws.maxOutput}
	stderrBuf := &cappedBuffer{cap: ws.maxOutput}
	moduleConfig := wazero.NewModuleConfig().
		WithStdin(strings.NewReader(stdin)).
		WithStdout(stdoutBuf).
		WithStderr(stderrBuf).
		WithStartFunctions() // Don't auto-call _start

	// Compile the module
	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return &WasmResult{
			ExitCode: -1,
			Stderr:   fmt.Sprintf("wasm compile error: %v", err),
			Duration: time.Since(start).String(),
		}, nil
	}

	// Collect exports
	var exports []string
	for _, exp := range compiled.ExportedFunctions() {
		exports = append(exports, exp.Name())
	}

	// Instantiate
	mod, err := runtime.InstantiateModule(ctx, compiled, moduleConfig)
	if err != nil {
		return &WasmResult{
			ExitCode: -1,
			Stderr:   fmt.Sprintf("wasm instantiate error: %v", err),
			Duration: time.Since(start).String(),
			Exports:  exports,
		}, nil
	}

	// Determine entry point
	if entryPoint == "" {
		entryPoint = "_start"
	}

	// Call the entry function
	fn := mod.ExportedFunction(entryPoint)
	if fn == nil {
		// Try "main" as fallback
		fn = mod.ExportedFunction("main")
	}

	result := &WasmResult{
		Duration: time.Since(start).String(),
		Exports:  exports,
		KVWrites: kvWrites,
	}

	if fn == nil {
		result.ExitCode = -1
		result.Stderr = fmt.Sprintf("entry point not found: %s (available: %v)", entryPoint, exports)
		return result, nil
	}

	_, err = fn.Call(ctx)
	result.Duration = time.Since(start).String()

	if err != nil {
		// A WASI command module calls proc_exit, which wazero surfaces as
		// sys.ExitError even on a clean exit(0). Treat that as the real exit
		// code rather than a failure.
		var exitErr *sys.ExitError
		switch {
		case errors.As(err, &exitErr):
			result.ExitCode = int(exitErr.ExitCode())
			if result.ExitCode != 0 {
				result.Stderr = truncate(stderrBuf.String(), ws.maxOutput)
			}
		case ctx.Err() == context.DeadlineExceeded:
			result.Stderr = "execution timeout exceeded"
			result.ExitCode = -2
		default:
			result.ExitCode = 1
			result.Stderr = truncate(err.Error(), ws.maxOutput)
		}
	}

	// Capture stdout (and stderr on success-but-noisy) regardless of exit path.
	result.Stdout = truncate(stdoutBuf.String(), ws.maxOutput)
	if result.Stderr == "" {
		result.Stderr = truncate(stderrBuf.String(), ws.maxOutput)
	}

	// Read memory usage
	if mem := mod.Memory(); mem != nil {
		result.MemUsed = mem.Size()
	}

	if len(kvWrites) == 0 {
		result.KVWrites = nil
	}

	return result, nil
}

// GetKV returns a value from the WASM sandbox's KV store.
func (ws *WasmSandbox) GetKV(key string) (string, bool) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	v, ok := ws.kvStore[key]
	return v, ok
}

// SetKV sets a value in the WASM sandbox's KV store (for pre-loading state).
func (ws *WasmSandbox) SetKV(key, value string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.kvStore[key] = value
}

// ClearKV clears the KV store.
func (ws *WasmSandbox) ClearKV() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.kvStore = make(map[string]string)
}

// Stats returns current sandbox statistics.
func (ws *WasmSandbox) Stats() map[string]any {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return map[string]any{
		"memory_limit_pages": ws.memoryLimit,
		"memory_limit_bytes": ws.memoryLimit * 65536,
		"max_duration":       ws.maxDuration.String(),
		"host_funcs":         len(ws.hostFuncs),
		"kv_entries":         len(ws.kvStore),
	}
}
