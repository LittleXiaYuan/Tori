import { createWASMPluginClient, WASMPluginClientError } from "./wasm-plugin";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(
  actual: unknown,
  expected: unknown,
  message?: string,
): void {
  if (actual !== expected)
    throw new Error(
      message ||
        `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`,
    );
}

function assertDeepEqual(
  actual: unknown,
  expected: unknown,
  message?: string,
): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson)
    throw new Error(
      message || `expected ${actualJson} to deep equal ${expectedJson}`,
    );
}

const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];

function test(name: string, fn: () => Promise<void> | void): void {
  tests.push({ name, fn });
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
  status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

const hostABIPlan = {
  plan_ready: true,
  ready: false,
  status: "plan_only",
  enforcement_ready: false,
  writes_files: false,
  network_access: false,
  functions: [],
  summary: {
    function_count: 0,
    enabled_count: 0,
    ledger_kv: false,
    memory_search: false,
    http_fetch: false,
    env_get: false,
    allowed_host_count: 0,
    env_allowlist_count: 0,
  },
  resource_limits: {
    max_memory_mb: 64,
    timeout_seconds: 30,
    allowed_hosts: [],
    env_allowlist: [],
  },
  labels: ["host-abi", "plan-only"],
};

const hostABIGate = {
  execution_gate_ready: true,
  allows_execution: false,
  blocked: true,
  status: "blocked_until_host_abi_enforcement",
  enforcement_ready: false,
  writes_files: false,
  network_access: false,
  requested_functions: ["ledger_kv_get", "ledger_kv_put"],
  allowed_functions: ["log_write"],
  blocked_functions: ["ledger_kv_get", "ledger_kv_put"],
  reason:
    "plugin requests privileged Host ABI functions while enforcement_ready=false",
  labels: ["host-abi", "execution-gate", "blocked", "needs-enforcement"],
};

const moduleIntegrityGate = {
  integrity_gate_ready: true,
  allows_execution: true,
  blocked: false,
  status: "verified",
  expected_sha256:
    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  actual_sha256:
    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  module_path: "calculator.wasm",
  writes_files: false,
  network_access: false,
  reason: "registered SHA-256 matches local module bytes",
  labels: ["module-integrity", "execution-gate", "verified"],
};

const remoteInstallPlan = {
  pack_id: "yunque.pack.wasm-plugin",
  generated_at: "now",
  status: "plan_only",
  remote_install_plan_ready: true,
  remote_install_ready: false,
  download_ready: false,
  signature_verify_ready: false,
  downloads: false,
  installs_plugin: false,
  writes_files: false,
  network_access: false,
  plugin: {
    slug: "calculator-remote",
    name: "Calculator Remote",
    version: "0.2.0",
    runtime: "wazero",
    entrypoint: "_start",
    module_path: "calculator-remote.wasm",
  },
  package: {
    manifest_url: "https://packs.yunque.local/wasm/calculator-remote.json",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    expected_sha256:
      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    signature: "sig",
    signature_algorithm: "ed25519",
    public_key_id: "root",
    trust_root: "yunque-pack-root",
    manifest_artifact: "calculator-remote-remote-manifest.json",
    package_artifact: "calculator-remote.tgz",
    cache_key: "cache-key",
  },
  signature_verification: {
    pack_id: "yunque.pack.wasm-plugin",
  generated_at: "now",
    signature_verification_plan_ready: true,
    verification_gate_ready: false,
  signature_verify_ready: false,
    required: true,
    allows_install: false,
    blocked: true,
    status: "blocked_until_signature_verifier",
    algorithm: "ed25519",
    signature_provided: true,
    public_key_id_present: true,
    public_key_id: "root",
    trust_root: "yunque-pack-root",
    expected_sha256:
      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    expected_sha256_format_valid: true,
  canonical_payload_sha256: "payload-digest",
    artifact: "signature-verification.json",
  downloads: false,
  writes_files: false,
  network_access: false,
  checks: [],
  labels: ["signature-verification", "plan-only", "blocked"],
  },
  checks: [],
  artifacts: ["remote-install-plan.json", "signature-verification.json"],
  actions: [],
  labels: ["remote-install", "plan-only"],
};

const approvalPlan = {
  pack_id: "yunque.pack.wasm-plugin",
  generated_at: "now",
  status: "plan_only",
  approval_gate_plan_ready: true,
  approval_gate_ready: false,
  requires_approval: true,
  approval_queue_plan_ready: true,
  approval_queue_ready: false,
  writes_approval_queue: false,
  writes_files: false,
  downloads: false,
  network_access: false,
  installs_plugin: false,
  decision: "requires_approval",
  risk_tier: "high",
  requested_by: "operator",
  reason: "preview approval gate",
  plugin: remoteInstallPlan.plugin,
  package: remoteInstallPlan.package,
  signature_verification: remoteInstallPlan.signature_verification,
  approval_queue_entry: {
    pack_id: "yunque.pack.wasm-plugin",
    generated_at: "now",
    approval_queue_plan_ready: true,
  approval_queue_ready: false,
    writes_approval_queue: false,
    requires_approval: true,
    status: "blocked_until_approval_queue",
    queue_name: "wasm_remote_install",
  request_id: "wasm-remote-install-preview",
  request_key: "request-key",
  decision: "requires_approval",
    decision_states: ["pending", "approved", "denied", "expired"],
    risk_tier: "high",
    requested_by: "operator",
    reason: "preview approval gate",
    approvers: ["security"],
    required_fields: ["request_id", "decision"],
  plugin: remoteInstallPlan.plugin,
  package: remoteInstallPlan.package,
  signature_gate_status: "blocked_until_signature_verifier",
    canonical_payload_sha256: "payload-digest",
    artifact: "approval-queue-entry.json",
    downloads: false,
    writes_files: false,
    network_access: false,
  installs_plugin: false,
    checks: [],
    labels: ["approval-queue", "plan-only", "no-queue-write"],
  },
  checks: [],
  approvers: ["security"],
  artifacts: [
    "approval-gate-plan.json",
    "approval-queue-entry.json",
    "remote-install-plan.json",
    "signature-verification.json",
  ],
  actions: [],
  labels: ["remote-install", "approval-gate", "plan-only"],
  remote_install_plan_summary: remoteInstallPlan,
};

const approvalDecisionPlan = {
  pack_id: "yunque.pack.wasm-plugin",
  generated_at: "now",
  status: "plan_only",
  approval_decision_plan_ready: true,
  approval_decision_ready: false,
  applies_approval_decision: false,
  approval_queue_plan_ready: true,
  approval_queue_ready: false,
  writes_approval_queue: false,
  writes_files: false,
  downloads: false,
  network_access: false,
  installs_plugin: false,
  decision: "approved",
  decision_by: "security",
  decision_reason: "preview decision",
  request_id: "wasm-remote-install-preview",
  request_key: "request-key",
  would_allow_installer_continue: true,
  blocks_installer: false,
  plugin: remoteInstallPlan.plugin,
  package: remoteInstallPlan.package,
  signature_verification: remoteInstallPlan.signature_verification,
  approval_queue_entry: approvalPlan.approval_queue_entry,
  decision_plan: {
    pack_id: "yunque.pack.wasm-plugin",
    generated_at: "now",
    approval_decision_plan_ready: true,
  approval_decision_ready: false,
  applies_approval_decision: false,
    approval_queue_plan_ready: true,
    approval_queue_ready: false,
    writes_approval_queue: false,
    requires_approval: true,
    status: "decision_preview_approved_pending_apply",
    queue_name: "wasm_remote_install",
    request_id: "wasm-remote-install-preview",
    request_key: "request-key",
  decision_key: "decision-key",
    decision: "approved",
  decision_by: "security",
  decision_reason: "preview decision",
  would_allow_installer_continue: true,
  blocks_installer: false,
    required_fields: ["request_id", "request_key", "decision"],
    plugin: remoteInstallPlan.plugin,
    package: remoteInstallPlan.package,
    signature_gate_status: "blocked_until_signature_verifier",
    canonical_payload_sha256: "payload-digest",
    artifact: "approval-decision-plan.json",
    downloads: false,
    writes_files: false,
    network_access: false,
    installs_plugin: false,
    checks: [],
  actions: [],
    labels: ["approval-decision", "plan-only", "no-queue-write"],
  },
  checks: [],
  artifacts: [
    "approval-decision-plan.json",
    "approval-queue-entry.json",
    "approval-gate-plan.json",
    "remote-install-plan.json",
    "signature-verification.json",
  ],
  actions: [],
  labels: ["remote-install", "approval-decision-plan", "plan-only"],
  approval_gate_plan_summary: approvalPlan,
};

const approvalWritebackPlan = {
  pack_id: "yunque.pack.wasm-plugin",
  generated_at: "now",
  status: "plan_only",
  approval_writeback_plan_ready: true,
  approval_writeback_ready: false,
  approval_queue_plan_ready: true,
  approval_queue_ready: false,
  writes_approval_queue: false,
  approval_decision_plan_ready: true,
  approval_decision_ready: false,
  applies_approval_decision: false,
  writes_files: false,
  downloads: false,
  network_access: false,
  installs_plugin: false,
  decision: "approved",
  decision_by: "security",
  decision_reason: "preview decision",
  request_id: "wasm-remote-install-preview",
  request_key: "request-key",
  decision_key: "decision-key",
  would_allow_installer_continue: true,
  blocks_installer: false,
  installer_blocked_until_writeback: true,
  plugin: remoteInstallPlan.plugin,
  package: remoteInstallPlan.package,
  signature_verification: remoteInstallPlan.signature_verification,
  approval_queue_entry: approvalPlan.approval_queue_entry,
  decision_plan: approvalDecisionPlan.decision_plan,
  writeback_plan: {
    pack_id: "yunque.pack.wasm-plugin",
    generated_at: "now",
    approval_writeback_plan_ready: true,
  approval_writeback_ready: false,
    approval_queue_plan_ready: true,
    approval_queue_ready: false,
    writes_approval_queue: false,
    approval_decision_plan_ready: true,
    approval_decision_ready: false,
    applies_approval_decision: false,
    requires_approval: true,
    status: "writeback_preview_blocked_until_queue_persistence",
    queue_name: "wasm_remote_install",
    writeback_store: "approval_queue",
    queue_operation: "plan_upsert_queue_entry",
    decision_operation: "plan_append_decision",
    request_id: "wasm-remote-install-preview",
    request_key: "request-key",
    decision_key: "decision-key",
    decision: "approved",
    decision_by: "security",
    decision_reason: "preview decision",
    would_allow_installer_continue: true,
    blocks_installer: false,
    installer_blocked_until_writeback: true,
    required_fields: ["request_id", "request_key", "decision_key"],
    plugin: remoteInstallPlan.plugin,
    package: remoteInstallPlan.package,
    signature_gate_status: "blocked_until_signature_verifier",
    canonical_payload_sha256: "payload-digest",
    queue_artifact: "approval-queue-entry.json",
    decision_artifact: "approval-decision-plan.json",
    artifact: "approval-writeback-plan.json",
    downloads: false,
    writes_files: false,
    network_access: false,
    installs_plugin: false,
    checks: [],
    actions: [],
    labels: ["approval-writeback", "plan-only", "no-queue-write"],
  },
  checks: [],
  artifacts: [
    "approval-writeback-plan.json",
    "approval-decision-plan.json",
    "approval-queue-entry.json",
    "approval-gate-plan.json",
    "remote-install-plan.json",
    "signature-verification.json",
  ],
  actions: [],
  labels: ["remote-install", "approval-writeback-plan", "plan-only"],
  remote_install_plan_summary: remoteInstallPlan,
  approval_gate_plan_summary: approvalPlan,
};

const approvalQueueStore = {
  pack_id: "yunque.pack.wasm-plugin",
  queue_name: "wasm_remote_install",
  store: "pack-local-json",
  store_ready: true,
  record_count: 1,
  artifact: "approval-queue-store.json",
  writes_files: false,
  writes_approval_queue: false,
  writes_approval_queue_store: false,
  installer_writeback_ready: false,
  notes: ["pack-local queue store only"],
};

const approvalQueueRecord = {
  pack_id: "yunque.pack.wasm-plugin",
  queue_name: "wasm_remote_install",
  request_id: "wasm-remote-install-preview",
  request_key: "request-key",
  decision_key: "decision-key",
  decision: "approved",
  decision_by: "security",
  decision_reason: "preview decision",
  risk_tier: "high",
  requested_by: "operator",
  reason: "preview approval gate",
  status: "written_pending_installer_wiring",
  created_at: "now",
  updated_at: "now",
  approval_queue_store_ready: true,
  writes_approval_queue: true,
  writes_approval_queue_store: true,
  approval_writeback_ready: true,
  approval_queue_ready: true,
  approval_decision_ready: true,
  applies_approval_decision: true,
  installer_blocked_until_writeback: false,
  installer_blocked_until_installer_wiring: true,
  plugin: remoteInstallPlan.plugin,
  package: remoteInstallPlan.package,
  signature_gate_status: "blocked_until_signature_verifier",
  canonical_payload_sha256: "payload-digest",
  approval_queue_entry: approvalPlan.approval_queue_entry,
  decision_plan: approvalDecisionPlan.decision_plan,
  writeback_plan: approvalWritebackPlan.writeback_plan,
  store_artifact: "approval-queue-store.json",
  downloads: false,
  writes_files: false,
  network_access: false,
  installs_plugin: false,
  artifacts: [
    "approval-queue-store.json",
    "approval-queue-record.json",
    "approval-writeback-plan.json",
  ],
  labels: ["remote-install", "approval-queue-record", "pack-local-store"],
};

const approvalQueueWriteback = {
  pack_id: "yunque.pack.wasm-plugin",
  generated_at: "now",
  status: "approval_queue_written_pending_installer_wiring",
  approval_queue_store_ready: true,
  approval_writeback_plan_ready: true,
  approval_writeback_ready: true,
  approval_queue_ready: true,
  writes_approval_queue: true,
  writes_approval_queue_store: true,
  approval_decision_ready: true,
  applies_approval_decision: true,
  writes_files: false,
  downloads: false,
  network_access: false,
  installs_plugin: false,
  decision: "approved",
  decision_by: "security",
  decision_reason: "preview decision",
  request_id: "wasm-remote-install-preview",
  request_key: "request-key",
  decision_key: "decision-key",
  installer_blocked_until_writeback: false,
  installer_blocked_until_installer_wiring: true,
  approval_queue_record: approvalQueueRecord,
  approval_queue_store: approvalQueueStore,
  plan_summary: approvalWritebackPlan,
  checks: [],
  artifacts: [
    "approval-queue-store.json",
    "approval-queue-record.json",
    "approval-writeback-plan.json",
    "approval-decision-plan.json",
  ],
  actions: [],
  labels: ["remote-install", "approval-queue-writeback", "pack-local-store"],
};

const installerContinuationPlan = {
    pack_id: "yunque.pack.wasm-plugin",
    generated_at: "now",
    status: "plan_only_blocked_until_installer_wiring",
    installer_continuation_plan_ready: true,
    consumes_approval_queue_store: true,
    approval_queue_store_ready: true,
    approval_queue_record_found: true,
    approval_queue_ready: true,
    approval_decision_ready: true,
    approval_writeback_ready: true,
    applies_approval_decision: true,
    approval_approved: true,
    would_allow_installer_continue: true,
    blocks_installer: true,
    installer_ready: false,
    installer_blocked_until_installer_wiring: true,
    remote_install_ready: false,
    download_ready: false,
    signature_verify_ready: false,
    downloads: false,
    writes_files: false,
    network_access: false,
    installs_plugin: false,
    decision: "approved",
    decision_by: "security",
    decision_reason: "preview decision",
    request_id: "wasm-remote-install-preview",
    request_key: "request-key",
    decision_key: "decision-key",
    plugin: remoteInstallPlan.plugin,
    package: remoteInstallPlan.package,
    signature_gate_status: "blocked_until_signature_verifier",
    canonical_payload_sha256: "payload-digest",
    approval_queue_record: approvalQueueRecord,
    approval_queue_store: approvalQueueStore,
    installer_plan: {
      pack_id: "yunque.pack.wasm-plugin",
      generated_at: "now",
      installer_continuation_plan_ready: true,
      consumes_approval_queue_store: true,
      approval_queue_store_ready: true,
      approval_queue_record_found: true,
      approval_queue_ready: true,
      approval_decision_ready: true,
      approval_approved: true,
      would_allow_installer_continue: true,
      blocks_installer: true,
      installer_ready: false,
      installer_blocked_until_installer_wiring: true,
      status: "plan_only_blocked_until_installer_wiring",
      queue_name: "wasm_remote_install",
      request_id: "wasm-remote-install-preview",
      request_key: "request-key",
      decision_key: "decision-key",
      decision: "approved",
      required_fields: [
        "approval-queue-store.json",
        "approval-queue-record.json",
        "decision=approved",
        "signature_verify_ready=true",
        "download_ready=true",
        "installer_registration_ready=true",
      ],
      plugin: remoteInstallPlan.plugin,
      package: remoteInstallPlan.package,
      signature_gate_status: "blocked_until_signature_verifier",
      canonical_payload_sha256: "payload-digest",
      queue_store_artifact: "approval-queue-store.json",
      queue_record_artifact: "approval-queue-record.json",
      download_handoff_artifact: "installer-download-handoff-plan.json",
      registration_handoff_artifact: "installer-registration-handoff-plan.json",
      audit_handoff_artifact: "installer-audit-handoff-plan.json",
      artifact: "installer-continuation-plan.json",
      remote_install_ready: false,
      download_ready: false,
      signature_verify_ready: false,
      downloads: false,
      writes_files: false,
      network_access: false,
      installs_plugin: false,
      checks: [],
      actions: [],
      labels: ["remote-install", "installer-continuation", "plan-only"],
    },
    checks: [],
    artifacts: [
      "installer-continuation-plan.json",
      "installer-download-handoff-plan.json",
      "installer-registration-handoff-plan.json",
      "installer-audit-handoff-plan.json",
      "approval-queue-store.json",
      "approval-queue-record.json",
      "signature-verification.json",
      "remote-install-plan.json",
    ],
    actions: [],
    labels: ["remote-install", "installer-continuation", "plan-only"],
  };

test("WASMPluginClient reads status and plugin list with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWASMPluginClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status"))
        return jsonResponse({
          pack_id: "yunque.pack.wasm-plugin",
          stage: "pack-shell-before-runtime-hosts",
          runtime_ready: true,
          abi_plan_ready: true,
          abi_ready: false,
          host_abi_execution_gate_ready: true,
          host_abi_enforcement_ready: false,
          module_integrity_gate_ready: true,
          remote_install_plan_ready: true,
          remote_install_ready: false,
          signature_verification_plan_ready: true,
          signature_verify_ready: false,
          approval_gate_plan_ready: true,
          approval_gate_ready: false,
          approval_queue_plan_ready: true,
          approval_queue_ready: true,
          approval_queue_store_ready: true,
          approval_queue_store: approvalQueueStore,
          approval_decision_plan_ready: true,
          approval_decision_ready: true,
          approval_writeback_plan_ready: true,
          approval_writeback_ready: true,
          installer_continuation_plan_ready: true,
          installer_ready: false,
          installer_blocked_until_installer_wiring: true,
          plugin_count: 1,
          loaded_count: 0,
          capabilities: [
            "wasm.host_abi.plan",
            "wasm.host_abi.execution_gate",
            "wasm.module.integrity_gate",
            "wasm.remote_install.plan",
            "wasm.remote_install.signature_verification_plan",
            "wasm.remote_install.approval_queue_plan",
            "wasm.remote_install.approval_plan",
            "wasm.remote_install.approval_decision_plan",
            "wasm.remote_install.approval_writeback_plan",
            "wasm.remote_install.approval_queue_writeback",
            "wasm.remote_install.installer_continuation_plan",
          ],
        });
      return jsonResponse({
        plugins: [
          {
            slug: "calculator",
            name: "Calculator",
            version: "0.1.0",
            runtime: "wazero",
            entrypoint: "plugin_exec",
            module_path: "calculator.wasm",
            status: "installed",
            exec_count: 0,
            permissions: {
              ledger_kv: true,
              memory_search: false,
              http_fetch: false,
              max_memory_mb: 64,
              timeout_seconds: 30,
            },
          },
        ],
        count: 1,
      });
    },
  });

  const status = await client.status();
  const plugins = await client.plugins();

  assertEqual(status.pack_id, "yunque.pack.wasm-plugin");
  assertEqual(status.abi_plan_ready, true);
  assertEqual(status.abi_ready, false);
  assertEqual(status.host_abi_execution_gate_ready, true);
  assertEqual(status.host_abi_enforcement_ready, false);
  assertEqual(status.module_integrity_gate_ready, true);
  assertEqual(status.remote_install_plan_ready, true);
  assertEqual(status.remote_install_ready, false);
  assertEqual(status.signature_verification_plan_ready, true);
  assertEqual(status.signature_verify_ready, false);
  assertEqual(status.approval_gate_plan_ready, true);
  assertEqual(status.approval_gate_ready, false);
  assertEqual(status.approval_queue_plan_ready, true);
  assertEqual(status.approval_queue_ready, true);
  assertEqual(status.approval_queue_store_ready, true);
  assertEqual(status.approval_queue_store?.artifact, "approval-queue-store.json");
  assertEqual(status.approval_decision_plan_ready, true);
  assertEqual(status.approval_decision_ready, true);
  assertEqual(status.approval_writeback_plan_ready, true);
  assertEqual(status.approval_writeback_ready, true);
  assertEqual(status.installer_continuation_plan_ready, true);
  assertEqual(status.installer_ready, false);
  assertEqual(status.installer_blocked_until_installer_wiring, true);
  assert(status.capabilities.includes("wasm.host_abi.plan"));
  assert(status.capabilities.includes("wasm.host_abi.execution_gate"));
  assert(status.capabilities.includes("wasm.module.integrity_gate"));
  assert(status.capabilities.includes("wasm.remote_install.plan"));
  assert(
    status.capabilities.includes(
      "wasm.remote_install.signature_verification_plan",
    ),
  );
  assert(
    status.capabilities.includes("wasm.remote_install.approval_queue_plan"),
  );
  assert(status.capabilities.includes("wasm.remote_install.approval_plan"));
  assert(
    status.capabilities.includes("wasm.remote_install.approval_decision_plan"),
  );
  assert(
    status.capabilities.includes("wasm.remote_install.approval_writeback_plan"),
  );
  assert(
    status.capabilities.includes(
      "wasm.remote_install.approval_queue_writeback",
    ),
  );
  assert(
    status.capabilities.includes(
      "wasm.remote_install.installer_continuation_plan",
    ),
  );
  assertEqual(plugins.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/wasm-plugin/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/wasm-plugin/plugins");
  assertEqual(
    new Headers(calls[0]?.init?.headers).get("authorization"),
    "Bearer token-123",
  );
});

test("WASMPluginClient installs, loads, executes dry-run, plans remote signed installs, reads detail, and unloads", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWASMPluginClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/plugins") && init?.method === "POST")
        return jsonResponse(
          {
            plugin: { slug: "calculator", name: "Calculator" },
            status: "installed",
          },
          { status: 201 },
        );
      if (String(url).endsWith("/plugins/load"))
        return jsonResponse(
          {
            plugin: { slug: "calculator", status: "loaded" },
            status: "loaded",
          },
          { status: 202 },
        );
      if (String(url).endsWith("/execute"))
        return jsonResponse({
          result: {
            slug: "calculator",
            dry_run: true,
            entrypoint: "plugin_exec",
            success: true,
            exit_code: 0,
            plan: [],
            host_abi_plan: hostABIPlan,
            host_abi_gate: hostABIGate,
            module_integrity_gate: moduleIntegrityGate,
          },
        });
      if (String(url).endsWith("/remote-install/approval/plan"))
        return jsonResponse({ plan: approvalPlan });
      if (String(url).endsWith("/remote-install/approval/decision/plan"))
        return jsonResponse({ plan: approvalDecisionPlan });
      if (String(url).endsWith("/remote-install/approval/writeback/plan"))
        return jsonResponse({ plan: approvalWritebackPlan });
      if (String(url).endsWith("/remote-install/approval/queue/writeback"))
        return jsonResponse({ writeback: approvalQueueWriteback }, { status: 202 });
      if (String(url).endsWith("/remote-install/installer/continuation/plan"))
        return jsonResponse({ plan: installerContinuationPlan });
      if (String(url).endsWith("/remote-install/plan"))
        return jsonResponse({ plan: remoteInstallPlan });
      if (String(url).includes("/plugins/calculator"))
        return jsonResponse({
          plugin: { slug: "calculator", status: "loaded" },
        });
      return jsonResponse(
        {
          plugin: { slug: "calculator", status: "installed" },
          status: "unloaded",
        },
        { status: 202 },
      );
    },
  });

  const installed = await client.installPlugin({
    slug: "calculator",
    name: "Calculator",
    module_path: "calculator.wasm",
    dry_run: true,
  });
  const loaded = await client.load("calculator");
  const executed = await client.execute({
    slug: "calculator",
    input: "{}",
    dry_run: true,
  });
  const remotePlanned = await client.remoteInstallPlan({
    slug: "calculator-remote",
    name: "Calculator Remote",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    sha256: "0123456789abcdef",
    signature: "sig",
    public_key_id: "root",
  });
  const gatePlanned = await client.remoteInstallApprovalPlan({
    slug: "calculator-remote",
    name: "Calculator Remote",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    sha256: "0123456789abcdef",
    signature: "sig",
    public_key_id: "root",
    requested_by: "operator",
    reason: "preview approval gate",
    risk_tier: "high",
    approvers: ["security"],
  });
  const decisionPlanned = await client.remoteInstallApprovalDecisionPlan({
    slug: "calculator-remote",
    name: "Calculator Remote",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    sha256: "0123456789abcdef",
    signature: "sig",
    public_key_id: "root",
    requested_by: "operator",
    reason: "preview approval gate",
    risk_tier: "high",
    approvers: ["security"],
    request_id: "wasm-remote-install-preview",
    request_key: "request-key",
    decision: "approved",
    decision_by: "security",
    decision_reason: "preview decision",
  });
  const writebackPlanned = await client.remoteInstallApprovalWritebackPlan({
    slug: "calculator-remote",
    name: "Calculator Remote",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    sha256: "0123456789abcdef",
    signature: "sig",
    public_key_id: "root",
    requested_by: "operator",
    reason: "preview approval gate",
    risk_tier: "high",
    approvers: ["security"],
    request_id: "wasm-remote-install-preview",
    request_key: "request-key",
    decision: "approved",
    decision_by: "security",
    decision_reason: "preview decision",
  });
  const queueWriteback = await client.remoteInstallApprovalQueueWriteback({
    slug: "calculator-remote",
    name: "Calculator Remote",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    sha256: "0123456789abcdef",
    signature: "sig",
    public_key_id: "root",
    requested_by: "operator",
    reason: "preview approval gate",
    risk_tier: "high",
    approvers: ["security"],
    request_id: "wasm-remote-install-preview",
    request_key: "request-key",
    decision: "approved",
    decision_by: "security",
    decision_reason: "preview decision",
  });
  const installerPlanned = await client.remoteInstallInstallerContinuationPlan({
    request_key: "request-key",
  });
  const detail = await client.plugin("calculator");
  const unloaded = await client.unload("calculator");

  assertEqual(installed.status, "installed");
  assertEqual(loaded.status, "loaded");
  assertEqual(executed.result.entrypoint, "plugin_exec");
  assertEqual(executed.result.host_abi_plan.plan_ready, true);
  assertEqual(executed.result.host_abi_plan.enforcement_ready, false);
  assertEqual(executed.result.host_abi_plan.writes_files, false);
  assertEqual(executed.result.host_abi_gate.execution_gate_ready, true);
  assertEqual(executed.result.host_abi_gate.allows_execution, false);
  assertEqual(executed.result.host_abi_gate.enforcement_ready, false);
  assertEqual(executed.result.module_integrity_gate.integrity_gate_ready, true);
  assertEqual(executed.result.module_integrity_gate.status, "verified");
  assertEqual(remotePlanned.plan.remote_install_plan_ready, true);
  assertEqual(remotePlanned.plan.remote_install_ready, false);
  assertEqual(remotePlanned.plan.writes_files, false);
  assertEqual(
    remotePlanned.plan.signature_verification.signature_verification_plan_ready,
    true,
  );
  assertEqual(
    remotePlanned.plan.signature_verification.signature_verify_ready,
    false,
  );
  assertEqual(remotePlanned.plan.signature_verification.allows_install, false);
  assertEqual(gatePlanned.plan.approval_gate_plan_ready, true);
  assertEqual(gatePlanned.plan.approval_gate_ready, false);
  assertEqual(gatePlanned.plan.requires_approval, true);
  assertEqual(gatePlanned.plan.approval_queue_plan_ready, true);
  assertEqual(gatePlanned.plan.writes_approval_queue, false);
  assertEqual(
    gatePlanned.plan.signature_verification.signature_verification_plan_ready,
    true,
  );
  assertEqual(
    gatePlanned.plan.approval_queue_entry.approval_queue_plan_ready,
    true,
  );
  assertEqual(
    gatePlanned.plan.approval_queue_entry.writes_approval_queue,
    false,
  );
  assertEqual(decisionPlanned.plan.approval_decision_plan_ready, true);
  assertEqual(decisionPlanned.plan.approval_decision_ready, false);
  assertEqual(decisionPlanned.plan.applies_approval_decision, false);
  assertEqual(decisionPlanned.plan.writes_approval_queue, false);
  assertEqual(decisionPlanned.plan.decision, "approved");
  assertEqual(decisionPlanned.plan.would_allow_installer_continue, true);
  assertEqual(
    decisionPlanned.plan.decision_plan.artifact,
    "approval-decision-plan.json",
  );
  assertEqual(writebackPlanned.plan.approval_writeback_plan_ready, true);
  assertEqual(writebackPlanned.plan.approval_writeback_ready, false);
  assertEqual(writebackPlanned.plan.writes_approval_queue, false);
  assertEqual(writebackPlanned.plan.installer_blocked_until_writeback, true);
  assertEqual(
    writebackPlanned.plan.writeback_plan.artifact,
    "approval-writeback-plan.json",
  );
  assertEqual(queueWriteback.writeback.approval_queue_store_ready, true);
  assertEqual(queueWriteback.writeback.writes_approval_queue, true);
  assertEqual(queueWriteback.writeback.writes_approval_queue_store, true);
  assertEqual(queueWriteback.writeback.approval_writeback_ready, true);
  assertEqual(queueWriteback.writeback.downloads, false);
  assertEqual(queueWriteback.writeback.writes_files, false);
  assertEqual(
    queueWriteback.writeback.installer_blocked_until_writeback,
    false,
  );
  assertEqual(
    queueWriteback.writeback.installer_blocked_until_installer_wiring,
    true,
  );
  assertEqual(
    queueWriteback.writeback.approval_queue_store.artifact,
    "approval-queue-store.json",
  );
  assertEqual(installerPlanned.plan.installer_continuation_plan_ready, true);
  assertEqual(installerPlanned.plan.consumes_approval_queue_store, true);
  assertEqual(installerPlanned.plan.approval_queue_record_found, true);
  assertEqual(installerPlanned.plan.would_allow_installer_continue, true);
  assertEqual(installerPlanned.plan.installer_ready, false);
  assertEqual(installerPlanned.plan.downloads, false);
  assertEqual(installerPlanned.plan.writes_files, false);
  assertEqual(
    installerPlanned.plan.installer_plan.artifact,
    "installer-continuation-plan.json",
  );
  assertEqual(detail.plugin.slug, "calculator");
  assertEqual(unloaded.status, "unloaded");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/wasm-plugin/plugins");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(
    calls[0]?.init?.body,
    JSON.stringify({
      slug: "calculator",
      name: "Calculator",
      module_path: "calculator.wasm",
      dry_run: true,
    }),
  );
  assertEqual(
    calls[1]?.url,
    "http://localhost:9090/v1/wasm-plugin/plugins/load",
  );
  assertEqual(calls[1]?.init?.body, JSON.stringify({ slug: "calculator" }));
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/wasm-plugin/execute");
  assertEqual(
    calls[2]?.init?.body,
    JSON.stringify({ slug: "calculator", input: "{}", dry_run: true }),
  );
  assertEqual(
    calls[3]?.url,
    "http://localhost:9090/v1/wasm-plugin/remote-install/plan",
  );
  assertEqual(calls[3]?.init?.method, "POST");
  assertEqual(
    calls[3]?.init?.body,
    JSON.stringify({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
    }),
  );
  assertEqual(
    calls[4]?.url,
    "http://localhost:9090/v1/wasm-plugin/remote-install/approval/plan",
  );
  assertEqual(calls[4]?.init?.method, "POST");
  assertEqual(
    calls[4]?.init?.body,
    JSON.stringify({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
      requested_by: "operator",
      reason: "preview approval gate",
      risk_tier: "high",
      approvers: ["security"],
    }),
  );
  assertEqual(
    calls[5]?.url,
    "http://localhost:9090/v1/wasm-plugin/remote-install/approval/decision/plan",
  );
  assertEqual(calls[5]?.init?.method, "POST");
  assertEqual(
    calls[5]?.init?.body,
    JSON.stringify({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
      requested_by: "operator",
      reason: "preview approval gate",
      risk_tier: "high",
      approvers: ["security"],
      request_id: "wasm-remote-install-preview",
      request_key: "request-key",
      decision: "approved",
      decision_by: "security",
      decision_reason: "preview decision",
    }),
  );
  assertEqual(
    calls[6]?.url,
    "http://localhost:9090/v1/wasm-plugin/remote-install/approval/writeback/plan",
  );
  assertEqual(calls[6]?.init?.method, "POST");
  assertEqual(
    calls[6]?.init?.body,
    JSON.stringify({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
      requested_by: "operator",
      reason: "preview approval gate",
      risk_tier: "high",
      approvers: ["security"],
      request_id: "wasm-remote-install-preview",
      request_key: "request-key",
      decision: "approved",
      decision_by: "security",
      decision_reason: "preview decision",
    }),
  );
  assertEqual(
    calls[7]?.url,
    "http://localhost:9090/v1/wasm-plugin/remote-install/approval/queue/writeback",
  );
  assertEqual(calls[7]?.init?.method, "POST");
  assertEqual(
    calls[7]?.init?.body,
    JSON.stringify({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
      requested_by: "operator",
      reason: "preview approval gate",
      risk_tier: "high",
      approvers: ["security"],
      request_id: "wasm-remote-install-preview",
      request_key: "request-key",
      decision: "approved",
      decision_by: "security",
      decision_reason: "preview decision",
    }),
  );
  assertEqual(
    calls[8]?.url,
    "http://localhost:9090/v1/wasm-plugin/remote-install/installer/continuation/plan",
  );
  assertEqual(calls[8]?.init?.method, "POST");
  assertEqual(
    calls[8]?.init?.body,
    JSON.stringify({ request_key: "request-key" }),
  );
  assertEqual(
    calls[9]?.url,
    "http://localhost:9090/v1/wasm-plugin/plugins/calculator",
  );
  assertEqual(
    calls[10]?.url,
    "http://localhost:9090/v1/wasm-plugin/plugins/unload",
  );
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("WASMPluginClient exports plugin evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWASMPluginClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({
        pack_id: "yunque.pack.wasm-plugin",
        exported_at: "now",
        format: "json-wasm-plugin-evidence",
        files: [
          "plugin.json",
          "host-abi-plan.json",
          "module-integrity-gate.json",
          "remote-install-plan.json",
          "signature-verification.json",
          "approval-queue-entry.json",
          "approval-gate-plan.json",
          "approval-decision-plan.json",
          "approval-writeback-plan.json",
          "approval-queue-store.json",
          "approval-queue-record.json",
          "installer-continuation-plan.json",
          "installer-download-handoff-plan.json",
          "installer-registration-handoff-plan.json",
          "installer-audit-handoff-plan.json",
        ],
        plugin: { slug: "calculator" },
        plan: [],
        host_abi_plan: hostABIPlan,
        host_abi_gate: hostABIGate,
        module_integrity_gate: moduleIntegrityGate,
        remote_install_plan: remoteInstallPlan,
        signature_verification: remoteInstallPlan.signature_verification,
        approval_gate_plan: approvalPlan,
        approval_decision_plan: approvalDecisionPlan,
        approval_writeback_plan: approvalWritebackPlan,
        approval_queue_store: approvalQueueStore,
        approval_queue_record: approvalQueueRecord,
        installer_continuation_plan: installerContinuationPlan.installer_plan,
      });
    },
  });

  const evidence = await client.evidence("calculator");

  assertEqual(evidence.format, "json-wasm-plugin-evidence");
  assertDeepEqual(evidence.files, [
    "plugin.json",
    "host-abi-plan.json",
    "module-integrity-gate.json",
    "remote-install-plan.json",
    "signature-verification.json",
    "approval-queue-entry.json",
    "approval-gate-plan.json",
    "approval-decision-plan.json",
    "approval-writeback-plan.json",
    "approval-queue-store.json",
    "approval-queue-record.json",
    "installer-continuation-plan.json",
    "installer-download-handoff-plan.json",
    "installer-registration-handoff-plan.json",
    "installer-audit-handoff-plan.json",
  ]);
  assertEqual(evidence.host_abi_plan.status, "plan_only");
  assertEqual(evidence.host_abi_gate.enforcement_ready, false);
  assertEqual(evidence.host_abi_gate.blocked, true);
  assertEqual(evidence.module_integrity_gate.status, "verified");
  assertEqual(evidence.remote_install_plan.downloads, false);
  assertEqual(evidence.signature_verification.signature_verify_ready, false);
  assertEqual(evidence.signature_verification.allows_install, false);
  assertEqual(evidence.approval_gate_plan.requires_approval, true);
  assertEqual(evidence.approval_gate_plan.approval_queue_plan_ready, true);
  assertEqual(
    evidence.approval_gate_plan.approval_queue_entry.artifact,
    "approval-queue-entry.json",
  );
  assertEqual(evidence.approval_decision_plan.approval_decision_ready, false);
  assertEqual(
    evidence.approval_decision_plan.decision_plan.artifact,
    "approval-decision-plan.json",
  );
  assertEqual(evidence.approval_writeback_plan.approval_writeback_ready, false);
  assertEqual(
    evidence.approval_writeback_plan.writeback_plan.artifact,
    "approval-writeback-plan.json",
  );
  assertEqual(evidence.approval_queue_store.artifact, "approval-queue-store.json");
  assertEqual(
    evidence.approval_queue_record.store_artifact,
    "approval-queue-store.json",
  );
  assertEqual(evidence.installer_continuation_plan.installer_ready, false);
  assertEqual(
    evidence.installer_continuation_plan.artifact,
    "installer-continuation-plan.json",
  );
  assertEqual(
    calls[0]?.url,
    "http://localhost:9090/v1/wasm-plugin/evidence/calculator",
  );
});

test("WASMPluginClient throws WASMPluginClientError with nested gateway messages", async () => {
  const client = createWASMPluginClient({
    baseUrl: "http://localhost:9090",
    fetch: async () =>
      jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
  });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof WASMPluginClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "pack route is not enabled" });
    assertEqual(error.message, "pack route is not enabled");
  }

  const nestedClient = createWASMPluginClient({
    baseUrl: "http://localhost:9090",
    fetch: async () =>
      jsonResponse(
        { error: { code: "BAD_PLUGIN", message: "slug is required" } },
        { status: 400 },
      ),
  });

  try {
    await nestedClient.execute({ slug: "" });
    throw new Error("expected execute to reject");
  } catch (error) {
    assert(error instanceof WASMPluginClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "slug is required");
  }
});

let failures = 0;
for (const { name, fn } of tests) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failures > 0) {
  process.exitCode = 1;
} else {
  console.log(`1..${tests.length}`);
}
