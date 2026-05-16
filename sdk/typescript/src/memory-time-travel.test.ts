import { MemoryTimeTravelClientError, createMemoryTimeTravelClient } from "./memory-time-travel";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
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

const snapshot = {
  id: "baseline",
  namespace: "memory_snapshot",
  created_at: "2026-05-15T12:00:00Z",
  values: { goal: "ship" },
  hash: "h",
  size_bytes: 12,
  key_count: 1,
  version: 1,
};

test("MemoryTimeTravelClient reads status and snapshots with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status")) return jsonResponse({ pack_id: "yunque.pack.memory-time-travel", stage: "pack-shell-before-ledger-kv-history", snapshot_store_ready: true, temporal_query_ready: true, ledger_history_ready: false, temporal_kv_adapter_ready: true, native_kv_history_plan_ready: true, kv_history_migration_plan_ready: true, kv_history_cutover_plan_ready: true, dual_read_plan_ready: true, dual_read_parity_check_ready: false, dual_write_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false, migrates_kv_history: false, dual_read_ready: false, dual_write_ready: false, cutover_ready: false, merkle_verification_ready: false, approved_rollback_plan_ready: true, rollback_writeback_plan_ready: true, global_approval_enqueue_ready: false, rollback_writeback_ready: false, writes_ledger_kv: false, writes_temporal_kv: false, retention_plan_ready: true, retention_prune_plan_ready: true, retention_prune_ready: false, kv_audit_link_schema_ready: true, kv_audit_link_preview_ready: true, kv_audit_linkage_ready: false, snapshot_count: 1, namespace_count: 1, policy: {}, capabilities: [] });
      return jsonResponse({ snapshots: [snapshot], count: 1 });
    },
  });

  const status = await client.status();
  const snapshots = await client.snapshots("memory_snapshot");

  assertEqual(status.pack_id, "yunque.pack.memory-time-travel");
  assertEqual(snapshots.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/memory-time-travel/snapshots?namespace=memory_snapshot");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("MemoryTimeTravelClient saves snapshots, reconstructs, diffs, and builds rollback plans", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/snapshots") && init?.method === "POST") return jsonResponse({ snapshot, status: "saved" }, { status: 201 });
      if (String(url).endsWith("/snapshot-at")) return jsonResponse({ namespace: "memory_snapshot", at: "2026-05-15T12:00:00Z", snapshot, values: snapshot.values, matched_id: "baseline", status: "reconstructed" });
      if (String(url).endsWith("/diff")) return jsonResponse({ diff: { id: "memory-diff-1", pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", created_at: "now", stage: "pack-shell-before-ledger-kv-history", base_id: "baseline", target_id: "candidate", added_count: 1, removed_count: 0, changed_count: 0, drift_score: 50, risk_level: "high", entries: [], rollback_plan: ["delete token"] } });
      return jsonResponse({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", snapshot_id: "baseline", dry_run: true, action_count: 1, actions: ["load snapshot baseline"], status: "dry_run" } });
    },
  });

  const saved = await client.saveSnapshot({ id: "baseline", values: { goal: "ship" } });
  const at = await client.snapshotAt({ namespace: "memory_snapshot", at: "2026-05-15T12:00:00Z" });
  const diff = await client.diff({ namespace: "memory_snapshot", base_id: "baseline", target_id: "candidate" });
  const rollback = await client.rollbackPlan({ namespace: "memory_snapshot", snapshot_id: "baseline", dry_run: true });

  assertEqual(saved.status, "saved");
  assertEqual(at.status, "reconstructed");
  assertEqual(diff.diff.risk_level, "high");
  assertEqual(rollback.plan.status, "dry_run");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/snapshots");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/memory-time-travel/snapshot-at");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/memory-time-travel/diff");
  assertEqual(calls[3]?.url, "http://localhost:9090/v1/memory-time-travel/rollback-plan");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("MemoryTimeTravelClient builds approved rollback writeback plans without Ledger writes", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ plan: { pack_id: "yunque.pack.memory-time-travel", generated_at: "2026-05-15T13:00:00Z", stage: "pack-shell-before-approved-rollback-writeback", status: "approved_rollback_writeback_plan", namespace: "memory_snapshot", snapshot_id: "baseline", requested_by: "operator", reason: "restore known-good memory", approval_id: "approval-123", dry_run: true, approval_required: true, approval_request_plan_ready: true, approval_manager_bridge_plan_ready: true, global_approval_enqueue_ready: false, approved_rollback_plan_ready: true, rollback_writeback_plan_ready: true, rollback_writeback_ready: false, writes_ledger_kv: false, writes_temporal_kv: false, merkle_append_ready: false, audit_proof_link_ready: false, action_count: 1, preview_values: { goal: "ship" }, rollback_plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", snapshot_id: "baseline", dry_run: true, action_count: 1, actions: ["put goal=ship"], status: "dry_run" }, proposed_approval_request: { request_id: "approval-123", request_key: "approval-123", queue_name: "memory_time_travel_rollback", category: "data_mutation", risk_level: "high", summary: "Approve Memory Time Travel rollback to snapshot baseline", details: { snapshot_id: "baseline", writes_ledger_kv: false }, requester: "operator", reason: "restore known-good memory", required_fields: ["id", "risk_level"], decision_states: ["pending", "approved", "denied", "expired"], approval_manager_enqueue_ready: false, global_approval_enqueue_ready: false, action_release_ready: false, source_store: "pack-local-memory-snapshot", source_artifact: "approved-rollback-plan.json", payload: {} }, writeback_actions: [{ operation: "ledger_kv_put_versioned_preview", namespace: "memory_snapshot", key: "goal", value_hash: "h", value_bytes: 4, target_snapshot_id: "baseline", temporal_version: 1, audit_action: "memory_time_travel.rollback_writeback.plan", requires_approval: true, approval_id: "approval-123", generated_at: "2026-05-15T13:00:00Z" }], artifacts: ["approved-rollback-plan.json", "rollback-writeback-plan.json", "approval-request-plan.json"], actions: [], labels: ["plan-only"] } });
    },
  });

  const planned = await client.approvedRollbackPlan({ namespace: "memory_snapshot", snapshot_id: "baseline", requested_by: "operator", reason: "restore known-good memory", approval_id: "approval-123", dry_run: true });

  assertEqual(planned.plan.approved_rollback_plan_ready, true);
  assertEqual(planned.plan.rollback_writeback_ready, false);
  assertEqual(planned.plan.writes_ledger_kv, false);
  assertEqual(planned.plan.proposed_approval_request.risk_level, "high");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/rollback/approved-plan");
  assertEqual(calls[0]?.init?.method, "POST");
});

test("MemoryTimeTravelClient reads detail and exports evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("/snapshots/")) return jsonResponse({ snapshot });
      return jsonResponse({ pack_id: "yunque.pack.memory-time-travel", exported_at: "now", format: "json-memory-time-travel-evidence", files: ["snapshot.json", "approved-rollback-plan.json", "rollback-writeback-plan.json", "approval-request-plan.json", "retention-plan.json", "retention-prune-plan.json", "native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json", "kv-history-migration-preview.json", "kv-history-dual-read-parity.json", "kv-history-cutover-plan.json", "kv-history-cutover-readiness.json", "kv-history-dual-read-plan.json", "kv-history-dual-write-plan.json", "audit-links.json", "audit-link-preview.json", "audit-verification.json"], snapshot, history: [], approved_rollback_plan: { approved_rollback_plan_ready: true, rollback_writeback_ready: false, writes_ledger_kv: false }, rollback_writeback_plan: [{ key: "goal", requires_approval: true }], approval_request_plan: { risk_level: "high", global_approval_enqueue_ready: false }, retention_plan: { dry_run: true, candidate_count: 0, actions: [] }, retention_prune_plan: { dry_run: true, prune_ready: false, approval_required: false, selected_candidate_count: 0, actions: [] }, native_kv_history_plan: { native_kv_history_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false }, kv_history_migration_plan: [{ writes: false }], kv_history_index_plan: [{ name: "kv_history_namespace_key_version_uq" }], kv_history_migration_preview: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T15:30:00Z", source_namespace: "__kv_history__", native_table: "kv_history", scanned_document_count: 1, preview_row_count: 1, returned_row_count: 1, native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, uses_reserved_kv_namespace: true, rows: [] }, kv_history_dual_read_parity: { dual_read_parity_check_ready: true, dual_read_parity_ready: true, parity_passed: true, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false, mismatch_count: 0 }, kv_history_cutover_plan: { kv_history_cutover_plan_ready: true, cutover_ready: false, writes_native_kv_history: false, dual_read_ready: false, dual_write_ready: false }, kv_history_cutover_readiness: { cutover_readiness_check_ready: true, cutover_ready: false, passed_gate_count: 3, blocked_gate_count: 4, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false, gates: [] }, kv_history_dual_read_plan: { plan_ready: true, ready: false, switches_adapter: false }, kv_history_dual_write_plan: { plan_ready: true, ready: false, writes_ledger_kv: false, writes_native_kv_history: false }, kv_audit_link_schema: { schema_ready: true, linkage_ready: false, kv_audit_links: [] }, kv_audit_links: [], kv_audit_link_preview: { kv_audit_link_preview_ready: true, kv_audit_linkage_ready: false, linkage_ready: false, merkle_append_ready: false, writes_ledger_kv: false, writes_native_kv_history: false, candidate_link_count: 1, matched_link_count: 1, pending_link_count: 0, candidate_links: [{ key: "goal", proof_status: "candidate_matched", matched_by: "audit_seq+audit_hash" }] }, audit_verification: { ready: true, valid: true, invalid_index: -1, record_count: 1, checked_at: "now" } });
    },
  });

  const detail = await client.snapshot("baseline");
  const evidence = await client.evidence("baseline");

  assertEqual(detail.snapshot.id, "baseline");
  assertEqual(evidence.format, "json-memory-time-travel-evidence");
  assertDeepEqual(evidence.files, ["snapshot.json", "approved-rollback-plan.json", "rollback-writeback-plan.json", "approval-request-plan.json", "retention-plan.json", "retention-prune-plan.json", "native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json", "kv-history-migration-preview.json", "kv-history-dual-read-parity.json", "kv-history-cutover-plan.json", "kv-history-cutover-readiness.json", "kv-history-dual-read-plan.json", "kv-history-dual-write-plan.json", "audit-links.json", "audit-link-preview.json", "audit-verification.json"]);
  assertEqual(evidence.approved_rollback_plan?.rollback_writeback_ready, false);
  assertEqual(evidence.approval_request_plan?.global_approval_enqueue_ready, false);
  assertEqual(evidence.rollback_writeback_plan?.[0]?.requires_approval, true);
  assertEqual(evidence.retention_plan?.dry_run, true);
  assertEqual(evidence.retention_prune_plan?.prune_ready, false);
  assertEqual(evidence.native_kv_history_plan?.native_kv_history_ready, false);
  assertEqual(evidence.native_kv_history_plan?.writes_native_kv_history, false);
  assertEqual(evidence.kv_history_migration_preview?.native_kv_history_preview_ready, true);
  assertEqual(evidence.kv_history_migration_preview?.writes_native_kv_history, false);
  assertEqual(evidence.kv_history_dual_read_parity?.dual_read_parity_ready, true);
  assertEqual(evidence.kv_history_dual_read_parity?.switches_temporal_adapter, false);
  assertEqual(evidence.kv_history_cutover_plan?.kv_history_cutover_plan_ready, true);
  assertEqual(evidence.kv_history_cutover_plan?.cutover_ready, false);
  assertEqual(evidence.kv_history_cutover_readiness?.cutover_readiness_check_ready, true);
  assertEqual(evidence.kv_history_cutover_readiness?.cutover_ready, false);
  assertEqual(evidence.kv_history_dual_read_plan?.ready, false);
  assertEqual(evidence.kv_history_dual_write_plan?.writes_ledger_kv, false);
  assertEqual(evidence.kv_audit_link_schema?.schema_ready, true);
  assertDeepEqual(evidence.kv_audit_links, []);
  assertEqual(evidence.kv_audit_link_preview?.kv_audit_link_preview_ready, true);
  assertEqual(evidence.kv_audit_link_preview?.linkage_ready, false);
  assertEqual(evidence.kv_audit_link_preview?.writes_ledger_kv, false);
  assertEqual(evidence.kv_audit_link_preview?.candidate_links?.[0]?.matched_by, "audit_seq+audit_hash");
  assertEqual(evidence.audit_verification?.valid, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/snapshots/baseline");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/memory-time-travel/evidence/baseline");
});

test("MemoryTimeTravelClient builds retention dry-run plans", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T13:00:00Z", dry_run: true, status: "dry_run", policy: { retention_days: 30, max_versions_per_key: 100, max_snapshots_per_namespace: 100, max_snapshot_bytes: 262144, max_keys_per_snapshot: 256, evidence_max_snapshots: 20 }, cutoff_at: "2026-04-15T13:00:00Z", scopes: ["pack-local-snapshots"], snapshot_count: 1, keep_count: 1, candidate_count: 0, reclaimable_bytes: 0, temporal_history_ready: true, temporal_prune_ready: false, candidates: [], actions: ["no pack-local snapshot prune action required under the current policy"] } });
    },
  });

  const plan = await client.retentionPlan("memory_snapshot");

  assertEqual(plan.plan.dry_run, true);
  assertEqual(plan.plan.temporal_prune_ready, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/retention/plan?namespace=memory_snapshot");
  assertEqual(calls[0]?.init?.method, "GET");
});

test("MemoryTimeTravelClient builds retention prune approval plans", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T13:00:00Z", dry_run: true, status: "approval_plan", approval_required: true, prune_ready: false, temporal_prune_ready: false, candidate_count: 1, selected_candidate_count: 1, reclaimable_bytes: 12, action_count: 1, requested_by: "operator", reason: "policy review", retention_plan_generated_at: "2026-05-15T13:00:00Z", candidates: [{ id: "old-baseline", namespace: "memory_snapshot", created_at: "2026-05-01T00:00:00Z", hash: "h", size_bytes: 12, key_count: 1, reasons: ["older_than_retention_days:7"], action: "would delete pack-local snapshot memory_snapshot/old-baseline" }], actions: ["requires approval before deleting pack-local snapshot memory_snapshot/old-baseline"] } });
    },
  });

  const plan = await client.retentionPrunePlan({ namespace: "memory_snapshot", candidate_ids: ["old-baseline"], requested_by: "operator", reason: "policy review", dry_run: true });

  assertEqual(plan.plan.approval_required, true);
  assertEqual(plan.plan.prune_ready, false);
  assertEqual(plan.plan.selected_candidate_count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/retention/prune-plan");
  assertEqual(calls[0]?.init?.method, "POST");
});

test("MemoryTimeTravelClient builds native kv_history table/index/migration plans", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T16:00:00Z", stage: "native-kv-history-plan-before-schema-migration", status: "plan_only", source: "temporal-kv-adapter-readiness", current_adapter: "reserved-ledger-kv-namespace", current_history_namespace: "__kv_history__", native_table: "kv_history", temporal_kv_adapter_ready: true, native_kv_history_plan_ready: true, kv_history_migration_plan_ready: true, kv_history_index_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false, migrates_kv_history: false, uses_reserved_kv_namespace: true, snapshot_store_ready: true, retention_plan_ready: true, audit_proof_link_schema_ready: true, schema_plan: [{ name: "namespace", type: "text", nullable: false, purpose: "Ledger KV namespace" }], kv_history_index_plan: [{ name: "kv_history_namespace_key_version_uq", columns: ["namespace", "key", "version"], unique: true, purpose: "idempotent replay" }], kv_history_migration_plan: [{ step: 1, name: "scan-reserved-ledger-kv-history", from: "__kv_history__", to: "migration-buffer", dry_run: true, writes: false, status: "planned", description: "scan without rewriting" }], artifacts: ["native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json"], actions: [], blocked_by: ["ledger-native-kv-history-schema-not-wired"], labels: ["plan-only"] } });
    },
  });

  const plan = await client.nativeKVHistoryPlan("memory_snapshot");

  assertEqual(plan.plan.native_kv_history_plan_ready, true);
  assertEqual(plan.plan.native_kv_history_ready, false);
  assertEqual(plan.plan.writes_native_kv_history, false);
  assertEqual(plan.plan.kv_history_migration_plan[0]?.writes, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/kv-history/native-plan?namespace=memory_snapshot");
  assertEqual(calls[0]?.init?.method, "GET");
});

test("MemoryTimeTravelClient previews native kv_history migration rows", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ kv_history_migration_preview: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T16:30:00Z", stage: "native-kv-history-migration-preview-before-native-write", status: "preview_only", source_namespace: "__kv_history__", native_table: "kv_history", scanned_document_count: 1, preview_row_count: 2, returned_row_count: 2, limit: 50, native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, uses_reserved_kv_namespace: true, rows: [{ id: "kvh-1", namespace: "memory_snapshot", key: "goal", version: 1, value_base64: "InNoaXAi", value_sha256: "h", updated_at: "2026-05-15T15:30:00Z", current: false, source_adapter: "reserved-ledger-kv-namespace" }], artifacts: ["kv-history-migration-preview.json"], labels: ["preview-only"] } });
    },
  });

  const preview = await client.nativeKVHistoryMigrationPreview("memory_snapshot", 50);

  assertEqual(preview.kv_history_migration_preview.native_kv_history_preview_ready, true);
  assertEqual(preview.kv_history_migration_preview.writes_native_kv_history, false);
  assertEqual(preview.kv_history_migration_preview.rows[0]?.source_adapter, "reserved-ledger-kv-namespace");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/kv-history/migration-preview?namespace=memory_snapshot&limit=50");
  assertEqual(calls[0]?.init?.method, "GET");
});

test("MemoryTimeTravelClient builds native kv_history cutover plans without switching adapters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T17:00:00Z", stage: "kv-history-cutover-plan-before-dual-read-write", status: "plan_only", dry_run: true, requested_by: "operator", reason: "cutover review", source: "native-plan-plus-migration-preview", native_table: "kv_history", current_history_namespace: "__kv_history__", consumes_native_kv_history_plan: true, consumes_migration_preview: true, native_kv_history_plan_ready: true, native_kv_history_preview_ready: true, kv_history_cutover_plan_ready: true, dual_read_plan_ready: true, dual_write_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false, migrates_kv_history: false, dual_read_ready: false, dual_write_ready: false, cutover_ready: false, rollback_ready: false, creates_native_table: false, deletes_reserved_kv_namespace: false, switches_temporal_adapter: false, preview_row_count: 1, returned_preview_row_count: 1, native_kv_history_plan: { native_kv_history_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false }, kv_history_migration_preview: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T15:30:00Z", source_namespace: "__kv_history__", native_table: "kv_history", scanned_document_count: 1, preview_row_count: 1, returned_row_count: 1, native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, uses_reserved_kv_namespace: true, rows: [] }, phases: [{ step: 1, name: "schema-readiness-gate", from: "native-kv-history-plan.json", to: "Ledger kv_history DDL", gate: "manual_schema_review", ready: false, writes: false, status: "blocked", description: "review" }], dual_read_plan: { plan_ready: true, ready: false, preferred_source: "native kv_history rows", fallback_source: "__kv_history__", reads_native_kv_history: false, reads_reserved_kv_namespace: true, switches_adapter: false, validation: ["shadow_read_parity"], blocked_by: ["dual-read-adapter-not-wired"] }, dual_write_plan: { plan_ready: true, ready: false, primary_target: "__kv_history__", mirror_target: "kv_history", writes_native_kv_history: false, writes_reserved_kv_namespace: false, writes_ledger_kv: false, migration_executor_ready: false, guardrails: ["approval"], blocked_by: ["dual-write-cutover-not-enabled"] }, cutover_rollback_plan: { plan_ready: true, ready: false, requires_approval: true, restores_reserved_adapter: true, drops_native_rows: false, deletes_reserved_kv_namespace: false, actions: ["restore reserved adapter"], blocked_by: ["cutover-rollback-executor-not-wired"] }, artifacts: ["kv-history-cutover-plan.json", "kv-history-dual-read-plan.json", "kv-history-dual-write-plan.json"], actions: [], blocked_by: ["dual-read-adapter-not-wired", "dual-write-cutover-not-enabled"], labels: ["plan-only"] } });
    },
  });

  const plan = await client.kvHistoryCutoverPlan({ namespace: "memory_snapshot", requested_by: "operator", reason: "cutover review", limit: 50, dry_run: true });

  assertEqual(plan.plan.kv_history_cutover_plan_ready, true);
  assertEqual(plan.plan.dual_read_plan_ready, true);
  assertEqual(plan.plan.dual_write_plan_ready, true);
  assertEqual(plan.plan.cutover_ready, false);
  assertEqual(plan.plan.writes_native_kv_history, false);
  assertEqual(plan.plan.dual_read_plan.switches_adapter, false);
  assertEqual(plan.plan.dual_write_plan.writes_ledger_kv, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/kv-history/cutover/plan");
  assertEqual(calls[0]?.init?.method, "POST");
});

test("MemoryTimeTravelClient runs native kv_history dual-read parity without switching adapters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ parity: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory-snapshot", temporal_namespace: "memory_snapshot", generated_at: "2026-05-15T18:00:00Z", at: "2026-05-15T18:00:00Z", stage: "kv-history-dual-read-parity-before-adapter-switch", status: "passed", source: "reserved-temporal-kv-versus-native-row-preview", native_table: "kv_history", current_history_namespace: "__kv_history__", limit: 500, preview_row_count: 2, returned_preview_row_count: 2, temporal_key_count: 2, native_preview_key_count: 2, matched_key_count: 2, mismatch_count: 0, missing_from_native_count: 0, extra_in_native_count: 0, value_mismatch_count: 0, dual_read_parity_check_ready: true, dual_read_parity_ready: true, parity_passed: true, preview_complete: true, reads_temporal_kv: true, reads_native_kv_history: false, reads_native_kv_history_preview: true, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false, kv_history_migration_preview: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T15:30:00Z", source_namespace: "__kv_history__", native_table: "kv_history", scanned_document_count: 1, preview_row_count: 1, returned_row_count: 1, native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, uses_reserved_kv_namespace: true, rows: [] }, mismatches: [], artifacts: ["kv-history-dual-read-parity.json"], actions: [], blocked_by: ["native-kv-history-read-adapter-not-wired"], labels: ["read-only"] } });
    },
  });

  const parity = await client.kvHistoryDualReadParity({ namespace: "memory_snapshot", at: "2026-05-15T18:00:00Z", limit: 500 });

  assertEqual(parity.parity.dual_read_parity_ready, true);
  assertEqual(parity.parity.parity_passed, true);
  assertEqual(parity.parity.switches_temporal_adapter, false);
  assertEqual(parity.parity.writes_ledger_kv, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/kv-history/dual-read/parity");
  assertEqual(calls[0]?.init?.method, "POST");
});


test("MemoryTimeTravelClient builds kv_history cutover readiness gates without switching adapters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ readiness: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory-snapshot", temporal_namespace: "memory_snapshot", generated_at: "2026-05-15T19:00:00Z", at: "2026-05-15T19:00:00Z", stage: "kv-history-cutover-readiness-before-adapter-switch", status: "blocked", dry_run: true, requested_by: "operator", reason: "readiness review", source: "cutover-plan-plus-dual-read-parity", native_table: "kv_history", current_history_namespace: "__kv_history__", cutover_readiness_check_ready: true, cutover_ready: false, native_kv_history_plan_ready: true, native_kv_history_preview_ready: true, dual_read_parity_check_ready: true, dual_read_parity_ready: true, parity_passed: true, preview_complete: true, migration_executor_ready: false, native_read_adapter_ready: false, native_write_path_ready: false, approval_manager_ready: false, rollback_executor_ready: false, audit_proof_link_ready: false, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false, consumes_cutover_plan: true, consumes_dual_read_parity: true, required_gate_count: 7, passed_gate_count: 3, blocked_gate_count: 4, gates: [{ name: "dual-read-parity", ready: true, required: true, status: "passed", evidence: ["kv-history-dual-read-parity.json"], description: "parity" }], cutover_plan: { kv_history_cutover_plan_ready: true, cutover_ready: false, writes_native_kv_history: false }, dual_read_parity: { dual_read_parity_ready: true, parity_passed: true, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false }, artifacts: ["kv-history-cutover-readiness.json"], actions: [], blocked_by: ["native-kv-history-read-adapter-not-wired"], labels: ["read-only"] } });
    },
  });

  const readiness = await client.kvHistoryCutoverReadiness({ namespace: "memory_snapshot", at: "2026-05-15T19:00:00Z", requested_by: "operator", reason: "readiness review", limit: 500, dry_run: true });

  assertEqual(readiness.readiness.cutover_readiness_check_ready, true);
  assertEqual(readiness.readiness.cutover_ready, false);
  assertEqual(readiness.readiness.parity_passed, true);
  assertEqual(readiness.readiness.switches_temporal_adapter, false);
  assertEqual(readiness.readiness.writes_ledger_kv, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/kv-history/cutover/readiness");
  assertEqual(calls[0]?.init?.method, "POST");
});

test("MemoryTimeTravelClient verifies Merkle audit chains", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ ready: true, valid: true, invalid_index: -1, record_count: 2, last_hash: "hash-2", last_seq: 2, checked_at: "2026-05-15T13:00:00Z", recent_records: [{ seq: 2, timestamp: "2026-05-15T13:00:00Z", type: "memory", action: "flush", hash: "hash-2" }] });
    },
  });

  const verify = await client.auditVerify(3);

  assertEqual(verify.ready, true);
  assertEqual(verify.valid, true);
  assertEqual(verify.last_hash, "hash-2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/audit/verify?limit=3");
  assertEqual(calls[0]?.init?.method, "GET");
});

test("MemoryTimeTravelClient reads KV audit proof-link schema placeholders", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ links: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T15:00:00Z", schema_ready: true, linkage_ready: false, native_kv_history_ready: false, merkle_verification_ready: false, source: "schema-placeholder-before-native-kv-history", kv_audit_links: [], required_fields: ["namespace", "key", "audit_hash", "proof_status"] } });
    },
  });

  const result = await client.auditLinks("memory_snapshot");

  assertEqual(result.links.schema_ready, true);
  assertEqual(result.links.linkage_ready, false);
  assertDeepEqual(result.links.kv_audit_links, []);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/audit/links?namespace=memory_snapshot");
  assertEqual(calls[0]?.init?.method, "GET");
});



test("MemoryTimeTravelClient previews KV audit proof links without writeback", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ preview: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", temporal_namespace: "memory_snapshot", generated_at: "2026-05-15T15:30:00Z", at: "2026-05-15T15:30:00Z", stage: "kv-audit-proof-link-preview-before-merkle-writeback", status: "preview_only", dry_run: true, source: "native-row-preview-plus-merkle-audit-records", native_table: "kv_history", preview_ready: true, linkage_ready: false, kv_audit_link_preview_ready: true, kv_audit_linkage_ready: false, native_kv_history_preview_ready: true, merkle_verification_ready: true, merkle_append_ready: false, writes_ledger_kv: false, writes_native_kv_history: false, merges_audit_proofs: false, limit: 50, preview_row_count: 1, returned_preview_row_count: 1, recent_audit_record_count: 1, candidate_link_count: 1, matched_link_count: 1, pending_link_count: 0, unmatched_row_count: 0, unmatched_audit_record_count: 0, schema: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T15:30:00Z", schema_ready: true, linkage_ready: false, preview_ready: true, native_kv_history_ready: false, merkle_verification_ready: true, source: "schema-plus-proof-link-preview", kv_audit_links: [], required_fields: ["namespace", "key", "audit_hash", "proof_status"] }, kv_history_migration_preview: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T15:30:00Z", source_namespace: "__kv_history__", native_table: "kv_history", scanned_document_count: 1, preview_row_count: 1, returned_row_count: 1, native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, uses_reserved_kv_namespace: true, rows: [] }, audit_verification: { ready: true, valid: true, invalid_index: -1, record_count: 1, checked_at: "2026-05-15T15:30:00Z" }, candidate_links: [{ namespace: "memory_snapshot", key: "goal", proof_status: "candidate_matched", matched_by: "audit_seq+audit_hash", native_row_id: "kvh-goal", audit_action: "memory.flush" }], unmatched_rows: [], unmatched_audit_records: [], artifacts: ["audit-link-preview.json", "audit-links.json", "audit-verification.json"], actions: [], blocked_by: ["per-kv-merkle-proof-link-not-wired"], labels: ["preview-only"] } });
    },
  });

  const result = await client.auditLinksPreview({ namespace: "memory_snapshot", at: "2026-05-15T15:30:00Z", limit: 50, dry_run: true });

  assertEqual(result.preview.kv_audit_link_preview_ready, true);
  assertEqual(result.preview.linkage_ready, false);
  assertEqual(result.preview.writes_ledger_kv, false);
  assertEqual(result.preview.merkle_append_ready, false);
  assertEqual(result.preview.candidate_links[0]?.matched_by, "audit_seq+audit_hash");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/audit/links/preview");
  assertEqual(calls[0]?.init?.method, "POST");
});

test("MemoryTimeTravelClient throws MemoryTimeTravelClientError with nested gateway messages", async () => {
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
  });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof MemoryTimeTravelClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "pack route is not enabled" });
    assertEqual(error.message, "pack route is not enabled");
  }

  const nestedClient = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_SNAPSHOT", message: "snapshot id is invalid" } }, { status: 400 }),
  });

  try {
    await nestedClient.saveSnapshot({ values: {} });
    throw new Error("expected saveSnapshot to reject");
  } catch (error) {
    assert(error instanceof MemoryTimeTravelClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "snapshot id is invalid");
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
