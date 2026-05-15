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
	if len(routes) != 10 {
		t.Fatalf("expected 10 WASM plugin routes, got %d", len(routes))
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
		"/v1/wasm-plugin/status":                                {http.MethodGet},
		"/v1/wasm-plugin/plugins":                               {http.MethodGet, http.MethodPost},
		"/v1/wasm-plugin/plugins/":                              {http.MethodGet},
		"/v1/wasm-plugin/plugins/load":                          {http.MethodPost},
		"/v1/wasm-plugin/plugins/unload":                        {http.MethodPost},
		"/v1/wasm-plugin/execute":                               {http.MethodPost},
		"/v1/wasm-plugin/remote-install/plan":                   {http.MethodPost},
		"/v1/wasm-plugin/remote-install/approval/plan":          {http.MethodPost},
		"/v1/wasm-plugin/remote-install/approval/decision/plan": {http.MethodPost},
		"/v1/wasm-plugin/evidence/":                             {http.MethodGet},
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
	if !dryRunResp.Result.ModuleIntegrityGate.IntegrityGateReady || dryRunResp.Result.ModuleIntegrityGate.Blocked || dryRunResp.Result.ModuleIntegrityGate.WritesFiles || dryRunResp.Result.ModuleIntegrityGate.NetworkAccess || dryRunResp.Result.ModuleIntegrityGate.Status != "pending_runtime_sha256" {
		t.Fatalf("dry-run should expose a non-destructive module integrity gate contract: %#v", dryRunResp.Result.ModuleIntegrityGate)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/plan", strings.NewReader(`{"slug":"calculator-remote","name":"Calculator Remote","version":"0.2.0","package_url":"https://packs.yunque.local/wasm/calculator-remote-0.2.0.tgz","manifest_url":"https://packs.yunque.local/wasm/calculator-remote.json","module_path":"calculator-remote.wasm","sha256":"0123456789abcdef","signature":"sig-ed25519","public_key_id":"yunque-root-2026","capabilities":["math.add"],"tags":["remote"]}`))
	h.RemoteInstallPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "remote_install_plan_ready") || !strings.Contains(w.Body.String(), "signature_verification_plan_ready") || !strings.Contains(w.Body.String(), "signature-verification.json") {
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
	if !remotePlanResp.Plan.SignatureVerification.SignatureVerificationPlanReady || remotePlanResp.Plan.SignatureVerification.VerificationGateReady || remotePlanResp.Plan.SignatureVerification.SignatureVerifyReady || remotePlanResp.Plan.SignatureVerification.AllowsInstall || !remotePlanResp.Plan.SignatureVerification.Blocked {
		t.Fatalf("remote install plan should expose a conservative signature verification gate preview: %#v", remotePlanResp.Plan.SignatureVerification)
	}
	if remotePlanResp.Plan.SignatureVerification.Algorithm != "ed25519" || remotePlanResp.Plan.SignatureVerification.Status != "blocked_invalid_sha256" || remotePlanResp.Plan.SignatureVerification.Downloads || remotePlanResp.Plan.SignatureVerification.WritesFiles || remotePlanResp.Plan.SignatureVerification.NetworkAccess || remotePlanResp.Plan.SignatureVerification.CanonicalPayloadSHA256 == "" {
		t.Fatalf("signature verification gate should be deterministic and non-destructive: %#v", remotePlanResp.Plan.SignatureVerification)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/wasm-plugin/status", nil)
	h.Status(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_queue_plan_ready") || !strings.Contains(w.Body.String(), "wasm.remote_install.approval_queue_plan") || !strings.Contains(w.Body.String(), "approval_decision_plan_ready") || !strings.Contains(w.Body.String(), "wasm.remote_install.approval_decision_plan") {
		t.Fatalf("status should expose approval queue/decision plan readiness and capabilities: status=%d body=%s", w.Code, w.Body.String())
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
	if !approvalPlanResp.Plan.ApprovalGatePlanReady || approvalPlanResp.Plan.ApprovalGateReady || !approvalPlanResp.Plan.RequiresApproval || !approvalPlanResp.Plan.ApprovalQueuePlanReady || approvalPlanResp.Plan.ApprovalQueueReady || approvalPlanResp.Plan.WritesApprovalQueue || approvalPlanResp.Plan.Downloads || approvalPlanResp.Plan.WritesFiles || approvalPlanResp.Plan.NetworkAccess || approvalPlanResp.Plan.InstallsPlugin {
		t.Fatalf("remote approval plan should be plan-only and non-destructive: %#v", approvalPlanResp.Plan)
	}
	if approvalPlanResp.Plan.Decision != "requires_approval" || approvalPlanResp.Plan.RiskTier != "critical" || approvalPlanResp.Plan.RequestedBy != "operator" || len(approvalPlanResp.Plan.Approvers) != 2 {
		t.Fatalf("remote approval plan should capture approval routing metadata: %#v", approvalPlanResp.Plan)
	}
	if !approvalPlanResp.Plan.SignatureVerification.SignatureVerificationPlanReady || approvalPlanResp.Plan.SignatureVerification.VerificationGateReady || approvalPlanResp.Plan.SignatureVerification.AllowsInstall {
		t.Fatalf("remote approval plan should embed the same signature verification gate preview: %#v", approvalPlanResp.Plan.SignatureVerification)
	}
	if !approvalPlanResp.Plan.ApprovalQueueEntry.ApprovalQueuePlanReady || approvalPlanResp.Plan.ApprovalQueueEntry.ApprovalQueueReady || approvalPlanResp.Plan.ApprovalQueueEntry.WritesApprovalQueue || approvalPlanResp.Plan.ApprovalQueueEntry.RequestID == "" || approvalPlanResp.Plan.ApprovalQueueEntry.RequestKey == "" || approvalPlanResp.Plan.ApprovalQueueEntry.Artifact != "approval-queue-entry.json" {
		t.Fatalf("remote approval plan should include deterministic approval queue entry preview: %#v", approvalPlanResp.Plan.ApprovalQueueEntry)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/approval/decision/plan", strings.NewReader(`{"slug":"calculator-remote","name":"Calculator Remote","version":"0.2.0","package_url":"https://packs.yunque.local/wasm/calculator-remote-0.2.0.tgz","manifest_url":"https://packs.yunque.local/wasm/calculator-remote.json","module_path":"calculator-remote.wasm","sha256":"0123456789abcdef","signature":"sig-ed25519","public_key_id":"yunque-root-2026","requested_by":"operator","reason":"test approval gate","risk_tier":"critical","approvers":["security","platform"],"request_id":"wasm-remote-install-custom","request_key":"custom-request-key","decision":"approved","decision_by":"security","decision_reason":"preview approval decision","metadata":{"ticket":"WASM-1"}}`))
	h.RemoteInstallApprovalDecisionPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_decision_plan_ready") || !strings.Contains(w.Body.String(), "approval-decision-plan.json") {
		t.Fatalf("remote install approval decision plan status=%d body=%s", w.Code, w.Body.String())
	}
	var decisionPlanResp struct {
		Plan RemoteInstallApprovalDecisionPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&decisionPlanResp); err != nil {
		t.Fatalf("decode remote install approval decision plan: %v", err)
	}
	if !decisionPlanResp.Plan.ApprovalDecisionPlanReady || decisionPlanResp.Plan.ApprovalDecisionReady || decisionPlanResp.Plan.AppliesApprovalDecision || decisionPlanResp.Plan.WritesApprovalQueue || decisionPlanResp.Plan.Downloads || decisionPlanResp.Plan.WritesFiles || decisionPlanResp.Plan.NetworkAccess || decisionPlanResp.Plan.InstallsPlugin {
		t.Fatalf("remote approval decision plan should be plan-only and non-destructive: %#v", decisionPlanResp.Plan)
	}
	if decisionPlanResp.Plan.Decision != "approved" || decisionPlanResp.Plan.DecisionBy != "security" || !decisionPlanResp.Plan.WouldAllowInstallerContinue || decisionPlanResp.Plan.BlocksInstaller {
		t.Fatalf("approved decision plan should preview later installer continuation without applying it: %#v", decisionPlanResp.Plan)
	}
	if decisionPlanResp.Plan.RequestID != "wasm-remote-install-custom" || decisionPlanResp.Plan.RequestKey != "custom-request-key" || decisionPlanResp.Plan.DecisionPlan.DecisionKey == "" || decisionPlanResp.Plan.DecisionPlan.Artifact != "approval-decision-plan.json" {
		t.Fatalf("decision plan should preserve queue references and artifact contract: %#v", decisionPlanResp.Plan.DecisionPlan)
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
	if !execResp.Result.ModuleIntegrityGate.IntegrityGateReady || !execResp.Result.ModuleIntegrityGate.AllowsExecution || execResp.Result.ModuleIntegrityGate.Status != "verified" || execResp.Result.ModuleIntegrityGate.ExpectedSHA256 == "" || execResp.Result.ModuleIntegrityGate.ActualSHA256 == "" {
		t.Fatalf("stateless execute should verify local module integrity before sandbox execution: %#v", execResp.Result.ModuleIntegrityGate)
	}

	if err := os.WriteFile(wasmPath, []byte("tampered wasm bytes"), 0o644); err != nil {
		t.Fatalf("tamper wasm: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/execute", strings.NewReader(`{"slug":"stateless","input":"tampered"}`))
	h.Execute(w, req)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "wasm module integrity blocked") || fake.calls != 1 {
		t.Fatalf("tampered execute should be blocked before sandbox execution, status=%d calls=%d body=%s", w.Code, fake.calls, w.Body.String())
	}
	var tamperedResp struct {
		Result ExecuteResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&tamperedResp); err != nil {
		t.Fatalf("decode tampered execute: %v", err)
	}
	if tamperedResp.Result.Success || tamperedResp.Result.ExitCode != -4 || !tamperedResp.Result.ModuleIntegrityGate.Blocked || tamperedResp.Result.ModuleIntegrityGate.Status != "blocked_module_sha256_mismatch" {
		t.Fatalf("unexpected tampered execute result: %#v", tamperedResp.Result)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/wasm-plugin/evidence/calculator", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-wasm-plugin-evidence") || !strings.Contains(w.Body.String(), "permission-plan.json") || !strings.Contains(w.Body.String(), "host-abi-plan.json") || !strings.Contains(w.Body.String(), "remote-install-plan.json") || !strings.Contains(w.Body.String(), "approval-gate-plan.json") || !strings.Contains(w.Body.String(), "approval-decision-plan.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
	var evidenceResp struct {
		Files                 []string                                `json:"files"`
		HostABIPlan           HostABIPlan                             `json:"host_abi_plan"`
		HostABIGate           HostABIExecutionGate                    `json:"host_abi_gate"`
		ModuleIntegrityGate   ModuleIntegrityGate                     `json:"module_integrity_gate"`
		RemoteInstallPlan     RemoteInstallPlanReport                 `json:"remote_install_plan"`
		SignatureVerification SignatureVerificationPlan               `json:"signature_verification"`
		ApprovalGatePlan      RemoteInstallApprovalPlanReport         `json:"approval_gate_plan"`
		ApprovalDecisionPlan  RemoteInstallApprovalDecisionPlanReport `json:"approval_decision_plan"`
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
	if !containsString(evidenceResp.Files, "module-integrity-gate.json") || !evidenceResp.ModuleIntegrityGate.IntegrityGateReady || evidenceResp.ModuleIntegrityGate.Status != "blocked_module_sha256_mismatch" {
		t.Fatalf("evidence should include module integrity gate state and artifact: files=%#v gate=%#v", evidenceResp.Files, evidenceResp.ModuleIntegrityGate)
	}
	if !evidenceResp.RemoteInstallPlan.RemoteInstallPlanReady || evidenceResp.RemoteInstallPlan.RemoteInstallReady {
		t.Fatalf("evidence should include remote install plan preview: %#v", evidenceResp.RemoteInstallPlan)
	}
	if !containsString(evidenceResp.Files, "signature-verification.json") || !evidenceResp.SignatureVerification.SignatureVerificationPlanReady || evidenceResp.SignatureVerification.SignatureVerifyReady || evidenceResp.SignatureVerification.AllowsInstall {
		t.Fatalf("evidence should include signature verification gate preview: files=%#v gate=%#v", evidenceResp.Files, evidenceResp.SignatureVerification)
	}
	if !evidenceResp.ApprovalGatePlan.ApprovalGatePlanReady || evidenceResp.ApprovalGatePlan.ApprovalGateReady || !evidenceResp.ApprovalGatePlan.RequiresApproval || !evidenceResp.ApprovalGatePlan.ApprovalQueuePlanReady {
		t.Fatalf("evidence should include approval gate plan preview: %#v", evidenceResp.ApprovalGatePlan)
	}
	if !containsString(evidenceResp.Files, "approval-decision-plan.json") || !evidenceResp.ApprovalDecisionPlan.ApprovalDecisionPlanReady || evidenceResp.ApprovalDecisionPlan.ApprovalDecisionReady || evidenceResp.ApprovalDecisionPlan.AppliesApprovalDecision {
		t.Fatalf("evidence should include approval decision plan preview: files=%#v plan=%#v", evidenceResp.Files, evidenceResp.ApprovalDecisionPlan)
	}
}

func TestWASMPluginRemoteInstallApprovalQueueEntryPlan(t *testing.T) {
	now := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}, Now: func() time.Time { return now }})
	body := `{"slug":"queued-calc","name":"Queued Calculator","version":"1.0.0","package_url":"https://packs.yunque.local/wasm/queued-calc-1.0.0.tgz","manifest_url":"https://packs.yunque.local/wasm/queued-calc.json","module_path":"queued-calc.wasm","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","signature":"ed25519:preview","signature_algorithm":"ed25519","public_key_id":"yunque-root-2026","trust_root":"yunque-root-bundle-2026","requested_by":"operator","reason":"queue preview","risk_tier":"critical","approvers":["security","platform"],"metadata":{"ticket":"WASM-Q-1"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/approval/plan", strings.NewReader(body))
	h.RemoteInstallApprovalPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_queue_entry") || !strings.Contains(w.Body.String(), "approval-queue-entry.json") {
		t.Fatalf("remote approval queue plan status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Plan RemoteInstallApprovalPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode remote approval queue plan: %v", err)
	}
	entry := resp.Plan.ApprovalQueueEntry
	if !resp.Plan.ApprovalQueuePlanReady || !entry.ApprovalQueuePlanReady || entry.ApprovalQueueReady || entry.WritesApprovalQueue {
		t.Fatalf("approval queue entry should be plan-only and not persisted: plan=%#v entry=%#v", resp.Plan, entry)
	}
	if entry.Status != "blocked_until_approval_queue" || entry.QueueName != "wasm_remote_install" || entry.RequestID == "" || entry.RequestKey == "" || entry.Decision != "requires_approval" || len(entry.DecisionStates) != 4 {
		t.Fatalf("approval queue entry should expose deterministic routing fields: %#v", entry)
	}
	if entry.RiskTier != "critical" || entry.RequestedBy != "operator" || entry.Reason != "queue preview" || len(entry.Approvers) != 2 || entry.Metadata["ticket"] != "WASM-Q-1" {
		t.Fatalf("approval queue entry should preserve approval metadata: %#v", entry)
	}
	if entry.Downloads || entry.WritesFiles || entry.NetworkAccess || entry.InstallsPlugin || entry.Artifact != "approval-queue-entry.json" {
		t.Fatalf("approval queue entry should be non-destructive: %#v", entry)
	}
}

func TestWASMPluginRemoteInstallApprovalDecisionPlan(t *testing.T) {
	now := time.Date(2026, 5, 15, 15, 0, 0, 0, time.UTC)
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}, Now: func() time.Time { return now }})
	base := `{"slug":"decision-calc","name":"Decision Calculator","version":"1.0.0","package_url":"https://packs.yunque.local/wasm/decision-calc-1.0.0.tgz","manifest_url":"https://packs.yunque.local/wasm/decision-calc.json","module_path":"decision-calc.wasm","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","signature":"ed25519:preview","signature_algorithm":"ed25519","public_key_id":"yunque-root-2026","trust_root":"yunque-root-bundle-2026","requested_by":"operator","reason":"decision preview","risk_tier":"critical","approvers":["security","platform"],"request_id":"wasm-remote-install-manual","request_key":"manual-request-key","decision_by":"security","decision_reason":"manual preview","metadata":{"ticket":"WASM-D-1"}`

	for _, tc := range []struct {
		decision string
		allows   bool
	}{
		{decision: "approved", allows: true},
		{decision: "denied", allows: false},
		{decision: "expired", allows: false},
	} {
		body := base + `,"decision":"` + tc.decision + `"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/approval/decision/plan", strings.NewReader(body))
		h.RemoteInstallApprovalDecisionPlan(w, req)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_decision_plan_ready") || !strings.Contains(w.Body.String(), "approval-decision-plan.json") {
			t.Fatalf("%s decision plan status=%d body=%s", tc.decision, w.Code, w.Body.String())
		}
		var resp struct {
			Plan RemoteInstallApprovalDecisionPlanReport `json:"plan"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode %s decision plan: %v", tc.decision, err)
		}
		plan := resp.Plan
		if !plan.ApprovalDecisionPlanReady || plan.ApprovalDecisionReady || plan.AppliesApprovalDecision || plan.WritesApprovalQueue || plan.Downloads || plan.WritesFiles || plan.NetworkAccess || plan.InstallsPlugin {
			t.Fatalf("%s decision plan should be plan-only and non-destructive: %#v", tc.decision, plan)
		}
		if plan.Decision != tc.decision || plan.DecisionPlan.Decision != tc.decision || plan.DecisionBy != "security" || plan.DecisionPlan.DecisionBy != "security" {
			t.Fatalf("%s decision plan should preserve decision metadata: %#v", tc.decision, plan)
		}
		if plan.WouldAllowInstallerContinue != tc.allows || plan.DecisionPlan.WouldAllowInstallerContinue != tc.allows || plan.BlocksInstaller == tc.allows || plan.DecisionPlan.BlocksInstaller == tc.allows {
			t.Fatalf("%s decision continuation/block policy mismatch: %#v", tc.decision, plan.DecisionPlan)
		}
		if plan.RequestID != "wasm-remote-install-manual" || plan.RequestKey != "manual-request-key" || plan.DecisionPlan.DecisionKey == "" || plan.DecisionPlan.Artifact != "approval-decision-plan.json" || plan.ApprovalQueueEntry.RequestID != "wasm-remote-install-manual" {
			t.Fatalf("%s decision plan should keep queue references and artifact: %#v", tc.decision, plan)
		}
		if plan.Metadata["ticket"] != "WASM-D-1" || plan.DecisionPlan.Metadata["ticket"] != "WASM-D-1" {
			t.Fatalf("%s decision plan should preserve metadata: %#v", tc.decision, plan)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/approval/decision/plan", strings.NewReader(base+`,"decision":"pending"}`))
	h.RemoteInstallApprovalDecisionPlan(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "decision must be approved, denied, or expired") {
		t.Fatalf("invalid decision should be rejected, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestWASMPluginRemoteInstallSignatureVerificationPlan(t *testing.T) {
	now := time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}, Now: func() time.Time { return now }})
	body := `{"slug":"signed-calc","name":"Signed Calculator","version":"1.0.0","package_url":"https://packs.yunque.local/wasm/signed-calc-1.0.0.tgz","manifest_url":"https://packs.yunque.local/wasm/signed-calc.json","module_path":"signed-calc.wasm","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","signature":"ed25519:preview","signature_algorithm":"Ed25519","public_key_id":"yunque-root-2026","trust_root":"yunque-root-bundle-2026"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/plan", strings.NewReader(body))
	h.RemoteInstallPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "signature_verification") || !strings.Contains(w.Body.String(), "signature_verification_plan_ready") {
		t.Fatalf("remote install signature plan status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Plan RemoteInstallPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode remote signature plan: %v", err)
	}
	sig := resp.Plan.SignatureVerification
	if !sig.SignatureVerificationPlanReady || sig.VerificationGateReady || sig.SignatureVerifyReady || !sig.Required || sig.AllowsInstall || !sig.Blocked {
		t.Fatalf("signature verification plan should be a blocking plan-only gate: %#v", sig)
	}
	if sig.Status != "blocked_until_signature_verifier" || sig.Algorithm != "ed25519" || !sig.SignatureProvided || !sig.PublicKeyIDPresent || !sig.ExpectedSHA256FormatValid {
		t.Fatalf("signature verification plan should normalize metadata and valid SHA-256: %#v", sig)
	}
	if sig.TrustRoot != "yunque-root-bundle-2026" || sig.CanonicalPayloadSHA256 == "" || sig.Downloads || sig.WritesFiles || sig.NetworkAccess || sig.Artifact != "signature-verification.json" {
		t.Fatalf("signature verification plan should remain deterministic and non-destructive: %#v", sig)
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

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
