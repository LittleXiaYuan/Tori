package wasmroute

// skill.go — the "tool line" for WASM packs (Tier 0 microkernel, see
// doc/MICROKERNEL-PACK-BLUEPRINT.md). WasmSkill makes a sandboxed WASM module
// callable by the planner as an agent tool: Skill.Execute marshals the tool args
// into the same request/response envelope ABI used by wasm routes, runs the
// module in the shared WasmSandbox, and returns its response body. This is what
// lets a *downloaded* WASM pack give the agent a new callable capability without
// recompiling the host.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/skills"
)

// WasmSkill is an agent tool backed by a sandboxed WASM module.
type WasmSkill struct {
	name        string
	description string
	params      map[string]any
	modulePath  string
	expectedSHA string
	entrypoint  string
	sb          *sandbox.WasmSandbox
	hostFuncs   []sandbox.ModuleHostFunc
}

// NewSkill builds a WASM-backed agent tool. sha256hex (optional) integrity-checks
// the module on each call; entrypoint defaults to "_start"; hostFuncs are the
// permission-scoped privileged imports exposed to the module for this tool.
func NewSkill(name, description string, params map[string]any, modulePath, sha256hex, entrypoint string, sb *sandbox.WasmSandbox, hostFuncs ...sandbox.ModuleHostFunc) *WasmSkill {
	ep := strings.TrimSpace(entrypoint)
	if ep == "" {
		ep = "_start"
	}
	if params == nil {
		params = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return &WasmSkill{
		name:        name,
		description: description,
		params:      params,
		modulePath:  modulePath,
		expectedSHA: strings.ToLower(strings.TrimSpace(sha256hex)),
		entrypoint:  ep,
		sb:          sb,
		hostFuncs:   hostFuncs,
	}
}

var _ skills.Skill = (*WasmSkill)(nil)

func (s *WasmSkill) Name() string                  { return s.name }
func (s *WasmSkill) Description() string            { return s.description }
func (s *WasmSkill) Parameters() map[string]any     { return s.params }

// Execute dispatches the tool call into the sandboxed WASM module and returns
// its response-envelope body.
func (s *WasmSkill) Execute(ctx context.Context, args map[string]any, _ *skills.Environment) (string, error) {
	if s.sb == nil {
		return "", fmt.Errorf("wasm sandbox not configured")
	}
	wasmBytes, err := os.ReadFile(s.modulePath)
	if err != nil {
		return "", fmt.Errorf("wasm module not found: %w", err)
	}
	if s.expectedSHA != "" {
		sum := sha256.Sum256(wasmBytes)
		if hex.EncodeToString(sum[:]) != s.expectedSHA {
			return "", fmt.Errorf("wasm module integrity check failed")
		}
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("encode args: %w", err)
	}
	stdin, err := json.Marshal(RequestEnvelope{
		Method: "POST",
		Path:   "/skill/" + s.name,
		Body:   string(argsJSON),
	})
	if err != nil {
		return "", fmt.Errorf("encode request envelope: %w", err)
	}
	result, err := s.sb.ExecuteWithHostFuncs(ctx, wasmBytes, string(stdin), s.entrypoint, s.hostFuncs)
	if err != nil {
		return "", fmt.Errorf("wasm execution error: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("wasm module exited %d: %s", result.ExitCode, result.Stderr)
	}
	resp, err := parseResponse(result.Stdout)
	if err != nil {
		return "", fmt.Errorf("invalid response envelope: %w", err)
	}
	return resp.Body, nil
}
