// Package wasmroute bridges HTTP requests to sandboxed WASM modules shipped
// inside a .yqpack. It defines the request/response envelope ABI (see
// docs/spec/pack-wasm-abi.md) and builds an http.HandlerFunc that marshals a
// request into the module's stdin, runs it in the shared WasmSandbox, and
// parses the module's stdout back into an HTTP response.
package wasmroute

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/packruntime"
)

// maxRequestBody caps the request body forwarded to a module, independent of
// the gateway's own body limit, so a module always sees a bounded stdin.
const maxRequestBody = 1 << 20 // 1 MiB

// RequestEnvelope is the JSON written to the module's stdin.
type RequestEnvelope struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   map[string][]string `json:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body,omitempty"`
}

// ResponseEnvelope is the JSON the module must write to stdout.
type ResponseEnvelope struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body,omitempty"`
}

// BuildRouteHandler returns an http.HandlerFunc for one wasm-backed route. The
// module bytes are read and integrity-checked from installedDir on each
// request (compiled-module caching is a later optimization). entrypoint
// defaults to "_start".
func BuildRouteHandler(installedDir string, rt packruntime.BackendRuntime, spec packruntime.BackendRouteSpec, sb *sandbox.WasmSandbox) http.HandlerFunc {
	modulePath := filepath.Join(installedDir, filepath.FromSlash(rt.Module))
	entrypoint := strings.TrimSpace(spec.Entrypoint)
	if entrypoint == "" {
		entrypoint = "_start"
	}
	expectedSHA := strings.ToLower(strings.TrimSpace(rt.SHA256))

	return func(w http.ResponseWriter, r *http.Request) {
		wasmBytes, err := os.ReadFile(modulePath)
		if err != nil {
			writeErr(w, http.StatusNotFound, "wasm module not found")
			return
		}
		if expectedSHA != "" {
			sum := sha256.Sum256(wasmBytes)
			if hex.EncodeToString(sum[:]) != expectedSHA {
				writeErr(w, http.StatusConflict, "wasm module integrity check failed")
				return
			}
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		envelope := RequestEnvelope{
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   r.URL.Query(),
			Headers: r.Header,
			Body:    string(body),
		}
		stdin, err := json.Marshal(envelope)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to encode request envelope")
			return
		}

		result, err := sb.Execute(r.Context(), wasmBytes, string(stdin), entrypoint)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("wasm execution error: %v", err))
			return
		}
		if result.ExitCode != 0 {
			writeErr(w, http.StatusBadGateway, fmt.Sprintf("wasm module exited %d: %s", result.ExitCode, result.Stderr))
			return
		}

		resp, err := parseResponse(result.Stdout)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "wasm module produced an invalid response envelope")
			return
		}
		writeResponse(w, resp)
	}
}

func parseResponse(stdout string) (ResponseEnvelope, error) {
	var resp ResponseEnvelope
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		return resp, fmt.Errorf("empty stdout")
	}
	if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
		return resp, err
	}
	if resp.Status == 0 {
		resp.Status = http.StatusOK
	}
	return resp, nil
}

func writeResponse(w http.ResponseWriter, resp ResponseEnvelope) {
	for k, vals := range resp.Headers {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.Status)
	_, _ = io.WriteString(w, resp.Body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
