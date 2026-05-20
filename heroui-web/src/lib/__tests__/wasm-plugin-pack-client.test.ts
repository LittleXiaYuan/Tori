import { afterEach, describe, expect, it, vi } from "vitest";
import { createWASMPluginPackClient } from "../wasm-plugin-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("wasm-plugin-pack-client", () => {
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

  const installerDownloadRecord = {
    pack_id: "yunque.pack.wasm-plugin",
    generated_at: "now",
    status: "download_written_pending_signature_verify",
    installer_download_writeback_ready: true,
    approval_queue_store_ready: true,
    approval_queue_record_found: true,
    approval_approved: true,
    download_ready: true,
    downloads: true,
    network_access: true,
    writes_files: false,
    writes_package_cache: true,
    signature_verify_ready: false,
    remote_install_ready: false,
    installs_plugin: false,
    installer_ready: false,
    installer_blocked_until_signature_verify: true,
    installer_blocked_until_registration: true,
    queue_name: "wasm_remote_install",
    request_id: "wasm-remote-install-preview",
    request_key: "request-key",
    decision_key: "decision-key",
    package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
    artifact: "installer-download-record.json",
    cache_artifact: "installer-package-cache-calculator-remote.tgz",
    cache_path: "/pack-data/installer-cache/installer-package-cache-calculator-remote.tgz",
    expected_sha256:
      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    actual_sha256:
      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    sha256_match: true,
    size_bytes: 42,
    plugin: remoteInstallPlan.plugin,
    package: remoteInstallPlan.package,
    checks: [],
    labels: ["remote-install", "installer-download", "pack-local-cache"],
  };

  const installerDownloadWriteback = {
    pack_id: "yunque.pack.wasm-plugin",
    generated_at: "now",
    status: "download_written_pending_signature_verify",
    installer_download_writeback_ready: true,
    consumes_approval_queue_store: true,
    consumes_installer_continuation_plan: true,
    approval_queue_store_ready: true,
    approval_queue_record_found: true,
    approval_approved: true,
    would_allow_installer_continue: true,
    approval_required: true,
    download_ready: true,
    downloads: true,
    network_access: true,
    writes_files: false,
    writes_package_cache: true,
    signature_verify_ready: false,
    remote_install_ready: false,
    installs_plugin: false,
    installer_ready: false,
    installer_blocked_until_signature_verify: true,
    installer_blocked_until_registration: true,
    request_id: "wasm-remote-install-preview",
    request_key: "request-key",
    decision_key: "decision-key",
    decision: "approved",
    approved_by: "security",
    reason: "download cache",
    plugin: remoteInstallPlan.plugin,
    package: remoteInstallPlan.package,
    approval_queue_record: approvalQueueRecord,
    approval_queue_store: approvalQueueStore,
    installer_continuation_plan: installerContinuationPlan.installer_plan,
    download_record: installerDownloadRecord,
    checks: [],
    artifacts: [
      "installer-download-record.json",
      "installer-package-cache-calculator-remote.tgz",
      "installer-continuation-plan.json",
    ],
    actions: [],
    labels: ["remote-install", "installer-download-writeback"],
  };

  it("reads WASM Plugin pack status and plugin metadata through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
            installer_download_writeback_ready: true,
            installer_ready: false,
            installer_blocked_until_signature_verify: true,
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
              "wasm.remote_install.installer_download_writeback",
            ],
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plugin: {
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
          }),
          { status: 200 },
        ),
      );

    const client = createWASMPluginPackClient();
    const status = await client.status();
    await client.plugins();
    await client.plugin("calculator");

    expect(status.abi_plan_ready).toBe(true);
    expect(status.abi_ready).toBe(false);
    expect(status.host_abi_execution_gate_ready).toBe(true);
    expect(status.host_abi_enforcement_ready).toBe(false);
    expect(status.module_integrity_gate_ready).toBe(true);
    expect(status.remote_install_plan_ready).toBe(true);
    expect(status.remote_install_ready).toBe(false);
    expect(status.signature_verification_plan_ready).toBe(true);
    expect(status.signature_verify_ready).toBe(false);
    expect(status.approval_gate_plan_ready).toBe(true);
    expect(status.approval_gate_ready).toBe(false);
    expect(status.approval_queue_plan_ready).toBe(true);
    expect(status.approval_queue_ready).toBe(true);
    expect(status.approval_queue_store_ready).toBe(true);
    expect(status.approval_queue_store?.artifact).toBe(
      "approval-queue-store.json",
    );
    expect(status.approval_decision_plan_ready).toBe(true);
    expect(status.approval_decision_ready).toBe(true);
    expect(status.approval_writeback_plan_ready).toBe(true);
    expect(status.approval_writeback_ready).toBe(true);
    expect(status.installer_continuation_plan_ready).toBe(true);
    expect(status.installer_download_writeback_ready).toBe(true);
    expect(status.installer_ready).toBe(false);
    expect(status.installer_blocked_until_signature_verify).toBe(true);
    expect(status.installer_blocked_until_installer_wiring).toBe(true);
    expect(status.capabilities).toContain("wasm.host_abi.plan");
    expect(status.capabilities).toContain("wasm.host_abi.execution_gate");
    expect(status.capabilities).toContain("wasm.module.integrity_gate");
    expect(status.capabilities).toContain("wasm.remote_install.plan");
    expect(status.capabilities).toContain(
      "wasm.remote_install.signature_verification_plan",
    );
    expect(status.capabilities).toContain(
      "wasm.remote_install.approval_queue_plan",
    );
    expect(status.capabilities).toContain("wasm.remote_install.approval_plan");
    expect(status.capabilities).toContain(
      "wasm.remote_install.approval_decision_plan",
    );
    expect(status.capabilities).toContain(
      "wasm.remote_install.approval_writeback_plan",
    );
    expect(status.capabilities).toContain(
      "wasm.remote_install.approval_queue_writeback",
    );
    expect(status.capabilities).toContain(
      "wasm.remote_install.installer_continuation_plan",
    );
    expect(status.capabilities).toContain(
      "wasm.remote_install.installer_download_writeback",
    );
    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/wasm-plugin/status",
      "/v1/wasm-plugin/plugins",
      "/v1/wasm-plugin/plugins/calculator",
    ]);
  });

  it("installs, loads, unloads, executes, and plans remote signed installs with method-aware payloads", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plugin: { slug: "calculator" },
            status: "installed",
          }),
          { status: 201 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plugin: { slug: "calculator", status: "loaded" },
            status: "loaded",
          }),
          { status: 202 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
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
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ plan: remoteInstallPlan }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ plan: approvalPlan }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ plan: approvalDecisionPlan }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ plan: approvalWritebackPlan }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ writeback: approvalQueueWriteback }), {
          status: 202,
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ plan: installerContinuationPlan }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ writeback: installerDownloadWriteback }),
          {
            status: 202,
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            plugin: { slug: "calculator", status: "installed" },
            status: "unloaded",
          }),
          { status: 202 },
        ),
      );

    const client = createWASMPluginPackClient();
    await client.installPlugin({
      slug: "calculator",
      name: "Calculator",
      module_path: "calculator.wasm",
      dry_run: true,
    });
    await client.load("calculator");
    const executed = await client.execute({
      slug: "calculator",
      input: "{}",
      dry_run: true,
    });
    const remotePlan = await client.remoteInstallPlan({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
    });
    const gatePlan = await client.remoteInstallApprovalPlan({
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
    const decisionPlan = await client.remoteInstallApprovalDecisionPlan({
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
    const writebackPlan = await client.remoteInstallApprovalWritebackPlan({
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
    const installerPlan = await client.remoteInstallInstallerContinuationPlan({
      request_key: "request-key",
    });
    const installerDownload = await client.remoteInstallInstallerDownloadWriteback({
      request_key: "request-key",
      approved: true,
      approved_by: "security",
      reason: "download cache",
    });
    await client.unload("calculator");

    expect(executed.result.host_abi_plan.plan_ready).toBe(true);
    expect(executed.result.host_abi_plan.enforcement_ready).toBe(false);
    expect(executed.result.host_abi_plan.writes_files).toBe(false);
    expect(executed.result.host_abi_gate.execution_gate_ready).toBe(true);
    expect(executed.result.host_abi_gate.allows_execution).toBe(false);
    expect(executed.result.host_abi_gate.enforcement_ready).toBe(false);
    expect(executed.result.module_integrity_gate.integrity_gate_ready).toBe(
      true,
    );
    expect(executed.result.module_integrity_gate.status).toBe("verified");
    expect(remotePlan.plan.remote_install_plan_ready).toBe(true);
    expect(remotePlan.plan.remote_install_ready).toBe(false);
    expect(remotePlan.plan.writes_files).toBe(false);
    expect(
      remotePlan.plan.signature_verification.signature_verification_plan_ready,
    ).toBe(true);
    expect(remotePlan.plan.signature_verification.signature_verify_ready).toBe(
      false,
    );
    expect(remotePlan.plan.signature_verification.allows_install).toBe(false);
    expect(gatePlan.plan.approval_gate_plan_ready).toBe(true);
    expect(gatePlan.plan.approval_gate_ready).toBe(false);
    expect(gatePlan.plan.requires_approval).toBe(true);
    expect(gatePlan.plan.approval_queue_plan_ready).toBe(true);
    expect(gatePlan.plan.writes_approval_queue).toBe(false);
    expect(
      gatePlan.plan.signature_verification.signature_verification_plan_ready,
    ).toBe(true);
    expect(gatePlan.plan.approval_queue_entry.approval_queue_plan_ready).toBe(
      true,
    );
    expect(gatePlan.plan.approval_queue_entry.writes_approval_queue).toBe(
      false,
    );
    expect(decisionPlan.plan.approval_decision_plan_ready).toBe(true);
    expect(decisionPlan.plan.approval_decision_ready).toBe(false);
    expect(decisionPlan.plan.applies_approval_decision).toBe(false);
    expect(decisionPlan.plan.writes_approval_queue).toBe(false);
    expect(decisionPlan.plan.decision).toBe("approved");
    expect(decisionPlan.plan.would_allow_installer_continue).toBe(true);
    expect(decisionPlan.plan.decision_plan.artifact).toBe(
      "approval-decision-plan.json",
    );
    expect(writebackPlan.plan.approval_writeback_plan_ready).toBe(true);
    expect(writebackPlan.plan.approval_writeback_ready).toBe(false);
    expect(writebackPlan.plan.writes_approval_queue).toBe(false);
    expect(writebackPlan.plan.installer_blocked_until_writeback).toBe(true);
    expect(writebackPlan.plan.writeback_plan.artifact).toBe(
      "approval-writeback-plan.json",
    );
    expect(queueWriteback.writeback.approval_queue_store_ready).toBe(true);
    expect(queueWriteback.writeback.writes_approval_queue).toBe(true);
    expect(queueWriteback.writeback.writes_approval_queue_store).toBe(true);
    expect(queueWriteback.writeback.approval_writeback_ready).toBe(true);
    expect(queueWriteback.writeback.downloads).toBe(false);
    expect(queueWriteback.writeback.writes_files).toBe(false);
    expect(
      queueWriteback.writeback.installer_blocked_until_writeback,
    ).toBe(false);
    expect(
      queueWriteback.writeback.installer_blocked_until_installer_wiring,
    ).toBe(true);
    expect(queueWriteback.writeback.approval_queue_store.artifact).toBe(
      "approval-queue-store.json",
    );
    expect(installerPlan.plan.installer_continuation_plan_ready).toBe(true);
    expect(installerPlan.plan.consumes_approval_queue_store).toBe(true);
    expect(installerPlan.plan.approval_queue_record_found).toBe(true);
    expect(installerPlan.plan.would_allow_installer_continue).toBe(true);
    expect(installerPlan.plan.installer_ready).toBe(false);
    expect(installerPlan.plan.downloads).toBe(false);
    expect(installerPlan.plan.writes_files).toBe(false);
    expect(installerPlan.plan.installer_plan.artifact).toBe(
      "installer-continuation-plan.json",
    );
    expect(
      installerDownload.writeback.installer_download_writeback_ready,
    ).toBe(true);
    expect(installerDownload.writeback.download_ready).toBe(true);
    expect(installerDownload.writeback.downloads).toBe(true);
    expect(installerDownload.writeback.network_access).toBe(true);
    expect(installerDownload.writeback.writes_package_cache).toBe(true);
    expect(installerDownload.writeback.writes_files).toBe(false);
    expect(installerDownload.writeback.signature_verify_ready).toBe(false);
    expect(installerDownload.writeback.remote_install_ready).toBe(false);
    expect(installerDownload.writeback.installs_plugin).toBe(false);
    expect(
      installerDownload.writeback.installer_blocked_until_signature_verify,
    ).toBe(true);
    expect(installerDownload.writeback.download_record.artifact).toBe(
      "installer-download-record.json",
    );
    expect(installerDownload.writeback.download_record.sha256_match).toBe(true);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/wasm-plugin/plugins");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[0]?.[1] as RequestInit).body)),
    ).toEqual({
      slug: "calculator",
      name: "Calculator",
      module_path: "calculator.wasm",
      dry_run: true,
    });
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/wasm-plugin/plugins/load");
    expect(
      JSON.parse(String((spy.mock.calls[1]?.[1] as RequestInit).body)),
    ).toEqual({ slug: "calculator" });
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/wasm-plugin/execute");
    expect(
      JSON.parse(String((spy.mock.calls[2]?.[1] as RequestInit).body)),
    ).toEqual({ slug: "calculator", input: "{}", dry_run: true });
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/wasm-plugin/remote-install/plan");
    expect((spy.mock.calls[3]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[3]?.[1] as RequestInit).body)),
    ).toEqual({
      slug: "calculator-remote",
      name: "Calculator Remote",
      package_url: "https://packs.yunque.local/wasm/calculator-remote.tgz",
      sha256: "0123456789abcdef",
      signature: "sig",
      public_key_id: "root",
    });
    expect(spy.mock.calls[4]?.[0]).toBe(
      "/v1/wasm-plugin/remote-install/approval/plan",
    );
    expect((spy.mock.calls[4]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[4]?.[1] as RequestInit).body)),
    ).toEqual({
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
    expect(spy.mock.calls[5]?.[0]).toBe(
      "/v1/wasm-plugin/remote-install/approval/decision/plan",
    );
    expect((spy.mock.calls[5]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[5]?.[1] as RequestInit).body)),
    ).toEqual({
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
    expect(spy.mock.calls[6]?.[0]).toBe(
      "/v1/wasm-plugin/remote-install/approval/writeback/plan",
    );
    expect((spy.mock.calls[6]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[6]?.[1] as RequestInit).body)),
    ).toEqual({
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
    expect(spy.mock.calls[7]?.[0]).toBe(
      "/v1/wasm-plugin/remote-install/approval/queue/writeback",
    );
    expect((spy.mock.calls[7]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[7]?.[1] as RequestInit).body)),
    ).toEqual({
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
    expect(spy.mock.calls[8]?.[0]).toBe(
      "/v1/wasm-plugin/remote-install/installer/continuation/plan",
    );
    expect((spy.mock.calls[8]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[8]?.[1] as RequestInit).body)),
    ).toEqual({ request_key: "request-key" });
    expect(spy.mock.calls[9]?.[0]).toBe(
      "/v1/wasm-plugin/remote-install/installer/download/writeback",
    );
    expect((spy.mock.calls[9]?.[1] as RequestInit).method).toBe("POST");
    expect(
      JSON.parse(String((spy.mock.calls[9]?.[1] as RequestInit).body)),
    ).toEqual({
      request_key: "request-key",
      approved: true,
      approved_by: "security",
      reason: "download cache",
    });
    expect(spy.mock.calls[10]?.[0]).toBe("/v1/wasm-plugin/plugins/unload");
  });

  it("exports JSON evidence packs by plugin slug", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
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
            "installer-download-record.json",
            "installer-package-cache.tgz",
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
          installer_download_record: installerDownloadRecord,
        }),
        { status: 200 },
      ),
    );

    const client = createWASMPluginPackClient();
    const evidence = await client.evidence("calculator");

    expect(evidence.files).toContain("host-abi-plan.json");
    expect(evidence.files).toContain("module-integrity-gate.json");
    expect(evidence.files).toContain("remote-install-plan.json");
    expect(evidence.files).toContain("signature-verification.json");
    expect(evidence.files).toContain("approval-gate-plan.json");
    expect(evidence.files).toContain("approval-decision-plan.json");
    expect(evidence.files).toContain("approval-writeback-plan.json");
    expect(evidence.files).toContain("approval-queue-store.json");
    expect(evidence.files).toContain("approval-queue-record.json");
    expect(evidence.files).toContain("installer-continuation-plan.json");
    expect(evidence.files).toContain("installer-download-handoff-plan.json");
    expect(evidence.files).toContain("installer-download-record.json");
    expect(evidence.files).toContain("installer-package-cache.tgz");
    expect(evidence.files).toContain("installer-registration-handoff-plan.json");
    expect(evidence.files).toContain("installer-audit-handoff-plan.json");
    expect(evidence.host_abi_plan.status).toBe("plan_only");
    expect(evidence.host_abi_gate.enforcement_ready).toBe(false);
    expect(evidence.host_abi_gate.blocked).toBe(true);
    expect(evidence.module_integrity_gate.status).toBe("verified");
    expect(evidence.remote_install_plan.downloads).toBe(false);
    expect(evidence.signature_verification.signature_verify_ready).toBe(false);
    expect(evidence.signature_verification.allows_install).toBe(false);
    expect(evidence.approval_gate_plan.requires_approval).toBe(true);
    expect(evidence.approval_gate_plan.approval_queue_plan_ready).toBe(true);
    expect(evidence.approval_gate_plan.approval_queue_entry.artifact).toBe(
      "approval-queue-entry.json",
    );
    expect(evidence.approval_decision_plan.approval_decision_ready).toBe(false);
    expect(evidence.approval_decision_plan.decision_plan.artifact).toBe(
      "approval-decision-plan.json",
    );
    expect(evidence.approval_writeback_plan.approval_writeback_ready).toBe(
      false,
    );
    expect(evidence.approval_writeback_plan.writeback_plan.artifact).toBe(
      "approval-writeback-plan.json",
    );
    expect(evidence.approval_queue_store.artifact).toBe(
      "approval-queue-store.json",
    );
    expect(evidence.approval_queue_record.store_artifact).toBe(
      "approval-queue-store.json",
    );
    expect(evidence.installer_continuation_plan.installer_ready).toBe(false);
    expect(evidence.installer_continuation_plan.artifact).toBe(
      "installer-continuation-plan.json",
    );
    expect(evidence.installer_download_record.artifact).toBe(
      "installer-download-record.json",
    );
    expect(evidence.installer_download_record.signature_verify_ready).toBe(
      false,
    );
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/wasm-plugin/evidence/calculator");
  });
});
