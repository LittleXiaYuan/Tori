package wasmplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/execution/sandbox"
)

type fakeWasmExecutor struct {
	calls int
	stats map[string]any
}

func (f *fakeWasmExecutor) Execute(ctx context.Context, wasmBytes []byte, stdin string, entryPoint string) (*sandbox.WasmResult, error) {
	f.calls++
	return &sandbox.WasmResult{ExitCode: 0, Stdout: stdin, Duration: "1ms", MemUsed: 1024, Exports: []string{entryPoint}, KVWrites: map[string]string{"last_input": stdin}}, nil
}

func (f *fakeWasmExecutor) Stats() map[string]any {
	if f.stats != nil {
		return f.stats
	}
	return map[string]any{"memory_limit_pages": uint32(1024), "max_duration": "30s"}
}

func TestWASMPluginHandlerRoutesExposePackShellSurface(t *testing.T) {
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	routes := h.Routes()
	if len(routes) != 9 {
		t.Fatalf("expected 9 WASM plugin routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	for _, route := range routes {
		methods := append([]string{}, route.Methods...)
		if route.Method != "" {
			methods = append([]string{route.Method}, methods...)
		}
		if route.Path == "" || route.Handler == nil || len(methods) == 0 {
			t.Fatalf("route must declare path, handler and method(s): %#v", route)
		}
		byPath[route.Path] = methods
	}
	expected := map[string][]string{
		"/v1/wasm-plugin/status":                       {http.MethodGet},
		"/v1/wasm-plugin/plugins":                      {http.MethodGet, http.MethodPost},
		"/v1/wasm-plugin/plugins/":                     {http.MethodGet},
		"/v1/wasm-plugin/plugins/load":                 {http.MethodPost},
		"/v1/wasm-plugin/plugins/unload":               {http.MethodPost},
		"/v1/wasm-plugin/execute":                      {http.MethodPost},
		"/v1/wasm-plugin/remote-install/plan":          {http.MethodPost},
		"/v1/wasm-plugin/remote-install/approval/plan": {http.MethodPost},
		"/v1/wasm-plugin/evidence/":                    {http.MethodGet},
	}
	for path, methods := range expected {
		if got, want := strings.Join(byPath[path], ","), strings.Join(methods, ","); got != want {
			t.Fatalf("expected %s methods %s, got %s", path, want, got)
		}
	}
}

func TestWASMPluginInstallLoadDryRunExecuteAndEvidence(t *testing.T) {
	pluginDir := t.TempDir()
	wasmPath := filepath.Join(pluginDir, "calculator.wasm")
	if err := os.WriteFile(wasmPath, []byte("fake wasm bytes"), 0o644); err != nil {
		t.Fatalf("write wasm: %v", err)
	}
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	fake := &fakeWasmExecutor{}
	h := New(Config{PluginDir: pluginDir, DataDir: t.TempDir(), Sandbox: fake, Now: func() time.Time { return now }})

	body := `{"slug":"calculator","name":"Calculator","module_path":"calculator.wasm","entrypoint":"plugin_exec","permissions":{"ledger_kv":true,"http_fetch":false,"max_memory_mb":32,"timeout_seconds":5},"capabilities":["math.add"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins", strings.NewReader(body))
	h.Plugins(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "calculator") || !strings.Contains(w.Body.String(), "sha256") {
		t.Fatalf("install status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins/load", strings.NewReader(`{"slug":"calculator"}`))
	h.Load(w, req)
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), "loaded") {
		t.Fatalf("load status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/execute", strings.NewReader(`{"slug":"calculator","input":"{\"a\":1}","dry_run":true}`))
	h.Execute(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "permission") || fake.calls != 0 {
		t.Fatalf("dry-run execute status=%d calls=%d body=%s", w.Code, fake.calls, w.Body.String())
	}
	var dryRunResp struct {
		Result ExecuteResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&dryRunResp); err != nil {
		t.Fatalf("decode dry-run execute: %v", err)
	}
	if !dryRunResp.Result.HostABIPlan.PlanReady || dryRunResp.Result.HostABIPlan.Ready || dryRunResp.Result.HostABIPlan.EnforcementReady {
		t.Fatalf("unexpected host ABI plan readiness: %#v", dryRunResp.Result.HostABIPlan)
	}
	if dryRunResp.Result.HostABIPlan.WritesFiles || dryRunResp.Result.HostABIPlan.Summary.EnabledCount == 0 {
		t.Fatalf("host ABI plan should be non-destructive and reflect enabled functions: %#v", dryRunResp.Result.HostABIPlan)
	}
	if !dryRunResp.Result.HostABIGate.ExecutionGateReady || dryRunResp.Result.HostABIGate.AllowsExecution || !dryRunResp.Result.HostABIGate.Blocked || dryRunResp.Result.HostABIGate.Status != "blocked_until_host_abi_enforcement" || len(dryRunResp.Result.HostABIGate.BlockedFunctions) == 0 {
		t.Fatalf("privileged host ABI dry-run should expose a blocking execution gate: %#v", dryRunResp.Result.HostABIGate)
	}
	if dryRunResp.Result.HostABIGate.EnforcementReady || dryRunResp.Result.HostABIGate.WritesFiles || dryRunResp.Result.HostABIGate.NetworkAccess {
		t.Fatalf("host ABI execution gate should remain non-destructive until enforcement is wired: %#v", dryRunResp.Result.HostABIGate)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/plan", strings.NewReader(`{"slug":"calculator-remote","name":"Calculator Remote","version":"0.2.0","package_url":"https://packs.yunque.local/wasm/calculator-remote-0.2.0.tgz","manifest_url":"https://packs.yunque.local/wasm/calculator-remote.json","module_path":"calculator-remote.wasm","sha256":"0123456789abcdef","signature":"sig-ed25519","public_key_id":"yunque-root-2026","capabilities":["math.add"],"tags":["remote"]}`))
	h.RemoteInstallPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "remote_install_plan_ready") || !strings.Contains(w.Body.String(), "signature-verification.json") {
		t.Fatalf("remote install plan status=%d body=%s", w.Code, w.Body.String())
	}
	var remotePlanResp struct {
		Plan RemoteInstallPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&remotePlanResp); err != nil {
		t.Fatalf("decode remote install plan: %v", err)
	}
	if !remotePlanResp.Plan.RemoteInstallPlanReady || remotePlanResp.Plan.RemoteInstallReady || remotePlanResp.Plan.Downloads || remotePlanResp.Plan.WritesFiles || remotePlanResp.Plan.NetworkAccess {
		t.Fatalf("remote install plan should be plan-only and non-destructive: %#v", remotePlanResp.Plan)
	}
	if remotePlanResp.Plan.SignatureVerifyReady || remotePlanResp.Plan.Package.Signature != "sig-ed25519" || remotePlanResp.Plan.Package.PublicKeyID != "yunque-root-2026" || remotePlanResp.Plan.Plugin.Slug != "calculator-remote" {
		t.Fatalf("remote install plan should capture signature metadata and plugin slug: %#v", remotePlanResp.Plan)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/approval/plan", strings.NewReader(`{"slug":"calculator-remote","name":"Calculator Remote","version":"0.2.0","package_url":"https://packs.yunque.local/wasm/calculator-remote-0.2.0.tgz","manifest_url":"https://packs.yunque.local/wasm/calculator-remote.json","module_path":"calculator-remote.wasm","sha256":"0123456789abcdef","signature":"sig-ed25519","public_key_id":"yunque-root-2026","requested_by":"operator","reason":"test approval gate","risk_tier":"critical","approvers":["security","platform"],"metadata":{"ticket":"WASM-1"}}`))
	h.RemoteInstallApprovalPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_gate_plan_ready") || !strings.Contains(w.Body.String(), "approval-gate-plan.json") {
		t.Fatalf("remote install approval plan status=%d body=%s", w.Code, w.Body.String())
	}
	var approvalPlanResp struct {
		Plan RemoteInstallApprovalPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&approvalPlanResp); err != nil {
		t.Fatalf("decode remote install approval plan: %v", err)
	}
	if !approvalPlanResp.Plan.ApprovalGatePlanReady || approvalPlanResp.Plan.ApprovalGateReady || !approvalPlanResp.Plan.RequiresApproval || approvalPlanResp.Plan.WritesApprovalQueue || approvalPlanResp.Plan.Downloads || approvalPlanResp.Plan.WritesFiles || approvalPlanResp.Plan.NetworkAccess || approvalPlanResp.Plan.InstallsPlugin {
		t.Fatalf("remote approval plan should be plan-only and non-destructive: %#v", approvalPlanResp.Plan)
	}
	if approvalPlanResp.Plan.Decision != "requires_approval" || approvalPlanResp.Plan.RiskTier != "critical" || approvalPlanResp.Plan.RequestedBy != "operator" || len(approvalPlanResp.Plan.Approvers) != 2 {
		t.Fatalf("remote approval plan should capture approval routing metadata: %#v", approvalPlanResp.Plan)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/execute", strings.NewReader(`{"slug":"calculator","input":"hello"}`))
	h.Execute(w, req)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "host ABI execution blocked") || fake.calls != 0 {
		t.Fatalf("privileged execute should be blocked before sandbox execution, status=%d calls=%d body=%s", w.Code, fake.calls, w.Body.String())
	}
	var blockedResp struct {
		Result ExecuteResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&blockedResp); err != nil {
		t.Fatalf("decode blocked execute: %v", err)
	}
	if blockedResp.Result.Success || blockedResp.Result.ExitCode != -3 || !blockedResp.Result.HostABIGate.Blocked || blockedResp.Result.HostABIGate.AllowsExecution {
		t.Fatalf("unexpected blocked execute result: %#v", blockedResp.Result)
	}

	statelessBody := `{"slug":"stateless","name":"Stateless","module_path":"calculator.wasm","entrypoint":"plugin_exec","permissions":{"ledger_kv":false,"memory_search":false,"http_fetch":false,"max_memory_mb":32,"timeout_seconds":5},"capabilities":["math.add"]}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins", strings.NewReader(statelessBody))
	h.Plugins(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "stateless") {
		t.Fatalf("install stateless status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins/load", strings.NewReader(`{"slug":"stateless"}`))
	h.Load(w, req)
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), "loaded") {
		t.Fatalf("load stateless status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/execute", strings.NewReader(`{"slug":"stateless","input":"hello"}`))
	h.Execute(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "last_input") || fake.calls != 1 {
		t.Fatalf("stateless execute status=%d calls=%d body=%s", w.Code, fake.calls, w.Body.String())
	}
	var execResp struct {
		Result ExecuteResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&execResp); err != nil {
		t.Fatalf("decode execute: %v", err)
	}
	if !execResp.Result.Success || execResp.Result.Stdout != "hello" || !execResp.Result.HostABIGate.AllowsExecution || execResp.Result.HostABIGate.Blocked || execResp.Result.HostABIGate.Status != "allowed_no_privileged_host_abi" {
		t.Fatalf("unexpected stateless execute result: %#v", execResp.Result)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/wasm-plugin/evidence/calculator", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-wasm-plugin-evidence") || !strings.Contains(w.Body.String(), "permission-plan.json") || !strings.Contains(w.Body.String(), "host-abi-plan.json") || !strings.Contains(w.Body.String(), "remote-install-plan.json") || !strings.Contains(w.Body.String(), "approval-gate-plan.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
	var evidenceResp struct {
		HostABIPlan       HostABIPlan                     `json:"host_abi_plan"`
		HostABIGate       HostABIExecutionGate            `json:"host_abi_gate"`
		RemoteInstallPlan RemoteInstallPlanReport         `json:"remote_install_plan"`
		ApprovalGatePlan  RemoteInstallApprovalPlanReport `json:"approval_gate_plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&evidenceResp); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if !evidenceResp.HostABIPlan.PlanReady || evidenceResp.HostABIPlan.Status != "plan_only" {
		t.Fatalf("evidence should include host ABI plan preview: %#v", evidenceResp.HostABIPlan)
	}
	if !evidenceResp.HostABIGate.ExecutionGateReady || !evidenceResp.HostABIGate.Blocked || evidenceResp.HostABIGate.EnforcementReady {
		t.Fatalf("evidence should include conservative Host ABI execution gate: %#v", evidenceResp.HostABIGate)
	}
	if !evidenceResp.RemoteInstallPlan.RemoteInstallPlanReady || evidenceResp.RemoteInstallPlan.RemoteInstallReady {
		t.Fatalf("evidence should include remote install plan preview: %#v", evidenceResp.RemoteInstallPlan)
	}
	if !evidenceResp.ApprovalGatePlan.ApprovalGatePlanReady || evidenceResp.ApprovalGatePlan.ApprovalGateReady || !evidenceResp.ApprovalGatePlan.RequiresApproval {
		t.Fatalf("evidence should include approval gate plan preview: %#v", evidenceResp.ApprovalGatePlan)
	}
}

func TestWASMPluginRejectsAbsoluteModulePath(t *testing.T) {
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins", strings.NewReader(`{"slug":"bad","name":"Bad","module_path":"C:/Windows/System32/bad.wasm"}`))
	h.Plugins(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for absolute module path, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestWASMPluginRejectsTraversalModulePath(t *testing.T) {
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}})
	for _, modulePath := range []string{"../secret.wasm", "nested/../../secret.wasm", `nested\..\..\secret.wasm`} {
		t.Run(modulePath, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins", strings.NewReader(`{"slug":"bad","name":"Bad","module_path":`+strconv.Quote(modulePath)+`}`))
			h.Plugins(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request for traversal module path, got %d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
