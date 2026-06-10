package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tetratelabs/wazero/api"

	"yunque-agent/internal/execution/sandbox"
)

// Privileged WASM host-function capabilities, gated by a pack's declared
// backend.permissions (docs/spec/pack-wasm-abi.md §6). An unpermitted capability
// is simply never exported to the module, so a pack cannot reach it at all.
const (
	// PermNetFetch lets a pack perform outbound HTTP via the host-side,
	// SSRF-guarded http_fetch host function.
	PermNetFetch = "net:fetch"
	// PermMemoryRead lets a pack recall the agent's memory for the request tenant
	// via memory_search.
	PermMemoryRead = "memory:read"
	// PermLedgerRead / PermLedgerWrite gate the pack-scoped persistent KV
	// (ledger_get / ledger_set), namespaced per pack + tenant.
	PermLedgerRead  = "ledger:read"
	PermLedgerWrite = "ledger:write"
)

// wasmHostFetchMaxResponse caps the response bytes a module can receive from
// http_fetch, independent of the gateway's own limits.
const wasmHostFetchMaxResponse = 256 * 1024

// wasmPackKVStore is the minimal persistent KV the pack-scoped ledger_* host
// functions need. *internal/ledger.KVConfigStore satisfies it.
type wasmPackKVStore interface {
	Put(ctx context.Context, key string, value any) error
	GetRaw(ctx context.Context, key string) ([]byte, error)
}

// SetWasmPackKVStore injects the persistent KV backing the pack-scoped
// ledger_get/ledger_set WASM host functions. Pass a store whose namespace is
// dedicated to pack data; keys are further namespaced per pack + tenant.
func (g *Gateway) SetWasmPackKVStore(kvs wasmPackKVStore) { g.wasmPackKV = kvs }

// buildWasmHostFuncs builds the permission-scoped privileged host functions for
// a pack. A capability is exported only when the pack declared its permission
// AND the backing provider is wired — registration itself is the enforcement
// boundary, so an unpermitted/unprovisioned capability is unreachable.
func (g *Gateway) buildWasmHostFuncs(packID string, permissions []string) []sandbox.ModuleHostFunc {
	perms := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		perms[strings.TrimSpace(p)] = true
	}

	var funcs []sandbox.ModuleHostFunc
	if perms[PermNetFetch] {
		funcs = append(funcs, sandbox.ModuleHostFunc{
			Name:    "http_fetch",
			Params:  4, // reqPtr, reqLen, respPtr, respCap
			Results: 1, // bytes written, or -(required size) when the buffer is too small
			Fn:      hostHTTPFetch,
		})
	}
	if perms[PermMemoryRead] && g.orchestrator != nil {
		funcs = append(funcs, sandbox.ModuleHostFunc{
			Name:    "memory_search",
			Params:  4,
			Results: 1,
			Fn:      g.hostMemorySearch,
		})
	}
	if perms[PermLedgerRead] && g.wasmPackKV != nil {
		funcs = append(funcs, sandbox.ModuleHostFunc{
			Name:    "ledger_get",
			Params:  4,
			Results: 1,
			Fn:      g.hostLedgerGet(packID),
		})
	}
	if perms[PermLedgerWrite] && g.wasmPackKV != nil {
		funcs = append(funcs, sandbox.ModuleHostFunc{
			Name:    "ledger_set",
			Params:  4,
			Results: 1,
			Fn:      g.hostLedgerSet(packID),
		})
	}
	return funcs
}

// wasmHostIO standardizes the read-request / write-response buffer dance shared
// by the privileged host functions. The handler returns the JSON value to write
// back; >=0 result = bytes written, <0 = -(required size) when respCap is short.
func wasmHostIO(m api.Module, stack []uint64, handle func(req []byte) any) {
	reqPtr := api.DecodeU32(stack[0])
	reqLen := api.DecodeU32(stack[1])
	respPtr := api.DecodeU32(stack[2])
	respCap := api.DecodeU32(stack[3])

	raw, ok := m.Memory().Read(reqPtr, reqLen)
	if !ok {
		raw = nil
	}
	out, _ := json.Marshal(handle(raw))
	if uint32(len(out)) > respCap {
		stack[0] = api.EncodeI32(-int32(len(out)))
		return
	}
	m.Memory().Write(respPtr, out)
	stack[0] = api.EncodeI32(int32(len(out)))
}

// hostMemorySearch implements env.memory_search: recall the agent's compiled
// memory context for the request tenant. Request JSON {"query"}; response JSON
// {"context"} (or {"error"}).
func (g *Gateway) hostMemorySearch(ctx context.Context, m api.Module, stack []uint64) {
	wasmHostIO(m, stack, func(req []byte) any {
		var q struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(req, &q); err != nil || strings.TrimSpace(q.Query) == "" {
			return map[string]any{"error": "memory_search: invalid request (need query)"}
		}
		tenant := tenantFromCtx(ctx)
		compiled := g.orchestrator.CompileContext(ctx, tenant, q.Query)
		return map[string]any{"context": compiled}
	})
}

// packKVKey namespaces a pack's key by pack id + tenant so packs (and tenants)
// cannot read each other's persisted values.
func packKVKey(packID, tenant, key string) string {
	return packID + ":" + tenant + ":" + key
}

func (g *Gateway) hostLedgerGet(packID string) func(context.Context, api.Module, []uint64) {
	return func(ctx context.Context, m api.Module, stack []uint64) {
		wasmHostIO(m, stack, func(req []byte) any {
			var q struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(req, &q); err != nil || strings.TrimSpace(q.Key) == "" {
				return map[string]any{"error": "ledger_get: invalid request (need key)"}
			}
			tenant := tenantFromCtx(ctx)
			data, err := g.wasmPackKV.GetRaw(ctx, packKVKey(packID, tenant, q.Key))
			if err != nil || data == nil {
				return map[string]any{"found": false}
			}
			return map[string]any{"found": true, "value": string(data)}
		})
	}
}

func (g *Gateway) hostLedgerSet(packID string) func(context.Context, api.Module, []uint64) {
	return func(ctx context.Context, m api.Module, stack []uint64) {
		wasmHostIO(m, stack, func(req []byte) any {
			var q struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal(req, &q); err != nil || strings.TrimSpace(q.Key) == "" {
				return map[string]any{"ok": false, "error": "ledger_set: invalid request (need key)"}
			}
			tenant := tenantFromCtx(ctx)
			if err := g.wasmPackKV.Put(ctx, packKVKey(packID, tenant, q.Key), q.Value); err != nil {
				return map[string]any{"ok": false, "error": err.Error()}
			}
			return map[string]any{"ok": true}
		})
	}
}

type wasmFetchRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type wasmFetchResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body,omitempty"`
	Error  string `json:"error,omitempty"`
}

// hostHTTPFetch implements env.http_fetch(reqPtr, reqLen, respPtr, respCap) -> i32.
// It reads a JSON request from guest memory, performs an SSRF-guarded outbound
// request, and writes a JSON response envelope back. Transport/SSRF failures are
// returned as a {status:0,error:...} envelope (a successful write), so the i32
// return only ever signals buffer sizing: >=0 bytes written, or -(required size).
func hostHTTPFetch(ctx context.Context, m api.Module, stack []uint64) {
	reqPtr := api.DecodeU32(stack[0])
	reqLen := api.DecodeU32(stack[1])
	respPtr := api.DecodeU32(stack[2])
	respCap := api.DecodeU32(stack[3])

	write := func(env wasmFetchResponse) {
		out, _ := json.Marshal(env)
		if uint32(len(out)) > respCap {
			stack[0] = api.EncodeI32(-int32(len(out)))
			return
		}
		m.Memory().Write(respPtr, out)
		stack[0] = api.EncodeI32(int32(len(out)))
	}

	raw, ok := m.Memory().Read(reqPtr, reqLen)
	if !ok {
		write(wasmFetchResponse{Error: "http_fetch: invalid request pointer"})
		return
	}
	var req wasmFetchRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		write(wasmFetchResponse{Error: "http_fetch: invalid request json: " + err.Error()})
		return
	}
	u, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil {
		write(wasmFetchResponse{Error: "http_fetch: invalid url"})
		return
	}
	if err := validateSSRFTarget(u); err != nil {
		write(wasmFetchResponse{Error: "http_fetch: " + err.Error()})
		return
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	var bodyReader io.Reader
	if req.Body != "" && method != http.MethodGet && method != http.MethodHead {
		bodyReader = strings.NewReader(req.Body)
	}
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, method, u.String(), bodyReader)
	if err != nil {
		write(wasmFetchResponse{Error: "http_fetch: " + err.Error()})
		return
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	client := newSSRFSafeClient(15 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		write(wasmFetchResponse{Error: "http_fetch: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, wasmHostFetchMaxResponse))
	slog.Debug("wasm host http_fetch", "url", u.Redacted(), "status", resp.StatusCode, "bytes", len(respBody))
	write(wasmFetchResponse{Status: resp.StatusCode, Body: string(respBody)})
}
