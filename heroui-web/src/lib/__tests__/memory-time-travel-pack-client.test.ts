import { afterEach, describe, expect, it, vi } from "vitest";
import { createMemoryTimeTravelPackClient } from "../memory-time-travel-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("memory-time-travel-pack-client", () => {
  it("reads Memory Time Travel status, snapshots, and detail through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.memory-time-travel", stage: "pack-shell-before-ledger-kv-history", snapshot_store_ready: true, temporal_query_ready: true, ledger_history_ready: false, temporal_kv_adapter_ready: true, native_kv_history_plan_ready: true, kv_history_migration_plan_ready: true, kv_history_cutover_plan_ready: true, dual_read_plan_ready: true, dual_read_parity_check_ready: false, dual_write_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false, migrates_kv_history: false, dual_read_ready: false, dual_write_ready: false, cutover_ready: false, merkle_verification_ready: false, approved_rollback_plan_ready: true, rollback_writeback_plan_ready: true, global_approval_enqueue_ready: false, rollback_writeback_ready: false, writes_ledger_kv: false, writes_temporal_kv: false, retention_plan_ready: true, retention_prune_plan_ready: true, retention_prune_ready: false, kv_audit_link_schema_ready: true, kv_audit_linkage_ready: false, snapshot_count: 1, namespace_count: 1, policy: {}, capabilities: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshots: [{ id: "baseline", namespace: "memory_snapshot", created_at: "now", hash: "h", size_bytes: 12, key_count: 2, version: 1 }], count: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshot: { id: "baseline", namespace: "memory_snapshot", created_at: "now", values: { goal: "ship" }, hash: "h", size_bytes: 12, key_count: 1, version: 1 } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    await client.status();
    await client.snapshots("memory_snapshot");
    await client.snapshot("baseline");

    expect(spy.mock.calls.map((call) => call[0])).toEqual([
      "/v1/memory-time-travel/status",
      "/v1/memory-time-travel/snapshots?namespace=memory_snapshot",
      "/v1/memory-time-travel/snapshots/baseline",
    ]);
  });

  it("saves snapshots, reconstructs snapshot-at, diffs, and builds rollback plans", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ snapshot: { id: "baseline", namespace: "memory_snapshot", created_at: "now", values: { goal: "ship" }, hash: "h", size_bytes: 12, key_count: 1, version: 1 }, status: "saved" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ namespace: "memory_snapshot", at: "2026-05-15T12:00:00Z", values: { goal: "ship" }, matched_id: "baseline", status: "reconstructed" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ diff: { id: "memory-diff-1", pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", created_at: "now", stage: "pack-shell-before-ledger-kv-history", base_id: "baseline", target_id: "candidate", added_count: 1, removed_count: 0, changed_count: 0, drift_score: 50, risk_level: "high", entries: [], rollback_plan: ["delete token"] } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", snapshot_id: "baseline", dry_run: true, action_count: 2, actions: ["load snapshot baseline"], status: "dry_run" } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    await client.saveSnapshot({ id: "baseline", values: { goal: "ship" } });
    await client.snapshotAt({ namespace: "memory_snapshot", at: "2026-05-15T12:00:00Z" });
    await client.diff({ namespace: "memory_snapshot", base_id: "baseline", target_id: "candidate" });
    await client.rollbackPlan({ namespace: "memory_snapshot", snapshot_id: "baseline", dry_run: true });

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/snapshots");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(spy.mock.calls[1]?.[0]).toBe("/v1/memory-time-travel/snapshot-at");
    expect(spy.mock.calls[2]?.[0]).toBe("/v1/memory-time-travel/diff");
    expect(spy.mock.calls[3]?.[0]).toBe("/v1/memory-time-travel/rollback-plan");
  });

  it("builds approved rollback writeback plans without mutating Ledger KV", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.memory-time-travel", generated_at: "2026-05-15T13:00:00Z", stage: "pack-shell-before-approved-rollback-writeback", status: "approved_rollback_writeback_plan", namespace: "memory_snapshot", snapshot_id: "baseline", requested_by: "operator", reason: "restore known good memory", approval_id: "approval-123", dry_run: true, approval_required: true, approval_request_plan_ready: true, approval_manager_bridge_plan_ready: true, global_approval_enqueue_ready: false, approved_rollback_plan_ready: true, rollback_writeback_plan_ready: true, rollback_writeback_ready: false, writes_ledger_kv: false, writes_temporal_kv: false, merkle_append_ready: false, audit_proof_link_ready: false, action_count: 1, preview_values: { goal: "ship" }, rollback_plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", snapshot_id: "baseline", dry_run: true, action_count: 1, actions: ["put goal=ship"], status: "dry_run" }, proposed_approval_request: { request_id: "approval-123", request_key: "approval-123", queue_name: "memory_time_travel_rollback", category: "data_mutation", risk_level: "high", summary: "Approve Memory Time Travel rollback to snapshot baseline", details: { snapshot_id: "baseline", writes_ledger_kv: false }, requester: "operator", reason: "restore known good memory", required_fields: ["id", "risk_level"], decision_states: ["pending", "approved", "denied", "expired"], approval_manager_enqueue_ready: false, global_approval_enqueue_ready: false, action_release_ready: false, source_store: "pack-local-memory-snapshot", source_artifact: "approved-rollback-plan.json", payload: {} }, writeback_actions: [{ operation: "ledger_kv_put_versioned_preview", namespace: "memory_snapshot", key: "goal", value_hash: "h", value_bytes: 4, target_snapshot_id: "baseline", temporal_version: 1, audit_action: "memory_time_travel.rollback_writeback.plan", requires_approval: true, approval_id: "approval-123", generated_at: "2026-05-15T13:00:00Z" }], artifacts: ["approved-rollback-plan.json", "rollback-writeback-plan.json", "approval-request-plan.json"], actions: [], labels: ["plan-only"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.approvedRollbackPlan({ namespace: "memory_snapshot", snapshot_id: "baseline", requested_by: "operator", reason: "restore known good memory", approval_id: "approval-123", dry_run: true });

    expect(result.plan.approved_rollback_plan_ready).toBe(true);
    expect(result.plan.rollback_writeback_ready).toBe(false);
    expect(result.plan.writes_ledger_kv).toBe(false);
    expect(result.plan.proposed_approval_request.risk_level).toBe("high");
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/rollback/approved-plan");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
  });

  it("exports JSON evidence packs by snapshot id", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.memory-time-travel", exported_at: "now", format: "json-memory-time-travel-evidence", files: ["snapshot.json", "approved-rollback-plan.json", "rollback-writeback-plan.json", "approval-request-plan.json", "retention-plan.json", "retention-prune-plan.json", "native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json", "kv-history-migration-preview.json", "kv-history-dual-read-parity.json", "kv-history-cutover-plan.json", "kv-history-cutover-readiness.json", "kv-history-dual-read-plan.json", "kv-history-dual-write-plan.json", "audit-links.json", "audit-verification.json"], snapshot: { id: "baseline", values: {} }, history: [], approved_rollback_plan: { approved_rollback_plan_ready: true, rollback_writeback_ready: false, writes_ledger_kv: false }, rollback_writeback_plan: [{ key: "goal", requires_approval: true }], approval_request_plan: { risk_level: "high", global_approval_enqueue_ready: false }, retention_plan: { dry_run: true, candidate_count: 0, actions: [] }, retention_prune_plan: { dry_run: true, prune_ready: false, approval_required: false, selected_candidate_count: 0, actions: [] }, native_kv_history_plan: { native_kv_history_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false }, kv_history_migration_plan: [{ writes: false }], kv_history_index_plan: [{ name: "kv_history_namespace_key_version_uq" }], kv_history_migration_preview: { native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, rows: [] }, kv_history_dual_read_parity: { dual_read_parity_check_ready: true, dual_read_parity_ready: true, parity_passed: true, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false, mismatch_count: 0 }, kv_history_cutover_plan: { kv_history_cutover_plan_ready: true, cutover_ready: false, writes_native_kv_history: false }, kv_history_cutover_readiness: { cutover_readiness_check_ready: true, cutover_ready: false, passed_gate_count: 3, blocked_gate_count: 4, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false, gates: [] }, kv_history_dual_read_plan: { plan_ready: true, ready: false, switches_adapter: false }, kv_history_dual_write_plan: { plan_ready: true, ready: false, writes_ledger_kv: false }, kv_audit_link_schema: { schema_ready: true, linkage_ready: false, kv_audit_links: [] }, kv_audit_links: [], audit_verification: { ready: true, valid: true, invalid_index: -1, record_count: 1, checked_at: "now" } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const evidence = await client.evidence("baseline");

    expect(evidence.audit_verification?.valid).toBe(true);
    expect(evidence.approved_rollback_plan?.rollback_writeback_ready).toBe(false);
    expect(evidence.approval_request_plan?.global_approval_enqueue_ready).toBe(false);
    expect(evidence.rollback_writeback_plan?.[0]?.requires_approval).toBe(true);
    expect(evidence.retention_plan?.dry_run).toBe(true);
    expect(evidence.retention_prune_plan?.prune_ready).toBe(false);
    expect(evidence.native_kv_history_plan?.native_kv_history_ready).toBe(false);
    expect(evidence.native_kv_history_plan?.writes_native_kv_history).toBe(false);
    expect(evidence.kv_history_migration_preview?.native_kv_history_preview_ready).toBe(true);
    expect(evidence.kv_history_migration_preview?.writes_native_kv_history).toBe(false);
    expect(evidence.kv_history_dual_read_parity?.dual_read_parity_ready).toBe(true);
    expect(evidence.kv_history_dual_read_parity?.switches_temporal_adapter).toBe(false);
    expect(evidence.kv_history_cutover_plan?.kv_history_cutover_plan_ready).toBe(true);
    expect(evidence.kv_history_cutover_plan?.cutover_ready).toBe(false);
    expect(evidence.kv_history_cutover_readiness?.cutover_readiness_check_ready).toBe(true);
    expect(evidence.kv_history_cutover_readiness?.cutover_ready).toBe(false);
    expect(evidence.kv_history_dual_read_plan?.ready).toBe(false);
    expect(evidence.kv_history_dual_write_plan?.writes_ledger_kv).toBe(false);
    expect(evidence.kv_audit_link_schema?.schema_ready).toBe(true);
    expect(evidence.kv_audit_links).toEqual([]);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/evidence/baseline");
  });

  it("builds retention dry-run plans through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T13:00:00Z", dry_run: true, status: "dry_run", policy: { retention_days: 30, max_versions_per_key: 100, max_snapshots_per_namespace: 100, max_snapshot_bytes: 262144, max_keys_per_snapshot: 256, evidence_max_snapshots: 20 }, cutoff_at: "2026-04-15T13:00:00Z", scopes: ["pack-local-snapshots"], snapshot_count: 1, keep_count: 1, candidate_count: 0, reclaimable_bytes: 0, temporal_history_ready: true, temporal_prune_ready: false, candidates: [], actions: ["no pack-local snapshot prune action required under the current policy"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.retentionPlan("memory_snapshot");

    expect(result.plan.dry_run).toBe(true);
    expect(result.plan.temporal_prune_ready).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/retention/plan?namespace=memory_snapshot");
  });

  it("builds retention prune approval plans through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T13:00:00Z", dry_run: true, status: "approval_plan", approval_required: true, prune_ready: false, temporal_prune_ready: false, candidate_count: 1, selected_candidate_count: 1, reclaimable_bytes: 12, action_count: 1, requested_by: "operator", reason: "policy review", retention_plan_generated_at: "2026-05-15T13:00:00Z", candidates: [{ id: "old-baseline", namespace: "memory_snapshot", created_at: "2026-05-01T00:00:00Z", hash: "h", size_bytes: 12, key_count: 1, reasons: ["older_than_retention_days:7"], action: "would delete pack-local snapshot memory_snapshot/old-baseline" }], actions: ["requires approval before deleting pack-local snapshot memory_snapshot/old-baseline"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.retentionPrunePlan({ namespace: "memory_snapshot", candidate_ids: ["old-baseline"], requested_by: "operator", reason: "policy review", dry_run: true });

    expect(result.plan.approval_required).toBe(true);
    expect(result.plan.prune_ready).toBe(false);
    expect(result.plan.selected_candidate_count).toBe(1);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/retention/prune-plan");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
  });

  it("builds native kv_history table/index/migration plans through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T16:00:00Z", stage: "native-kv-history-plan-before-schema-migration", status: "plan_only", source: "temporal-kv-adapter-readiness", current_adapter: "reserved-ledger-kv-namespace", current_history_namespace: "__kv_history__", native_table: "kv_history", temporal_kv_adapter_ready: true, native_kv_history_plan_ready: true, kv_history_migration_plan_ready: true, kv_history_index_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false, migrates_kv_history: false, uses_reserved_kv_namespace: true, snapshot_store_ready: true, retention_plan_ready: true, audit_proof_link_schema_ready: true, schema_plan: [{ name: "namespace", type: "text", nullable: false, purpose: "Ledger KV namespace" }], kv_history_index_plan: [{ name: "kv_history_namespace_key_version_uq", columns: ["namespace", "key", "version"], unique: true, purpose: "idempotent replay" }], kv_history_migration_plan: [{ step: 1, name: "scan-reserved-ledger-kv-history", from: "__kv_history__", to: "migration-buffer", dry_run: true, writes: false, status: "planned", description: "scan without rewriting" }], artifacts: ["native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json"], actions: [], blocked_by: ["ledger-native-kv-history-schema-not-wired"], labels: ["plan-only"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.nativeKVHistoryPlan("memory_snapshot");

    expect(result.plan.native_kv_history_plan_ready).toBe(true);
    expect(result.plan.native_kv_history_ready).toBe(false);
    expect(result.plan.writes_native_kv_history).toBe(false);
    expect(result.plan.artifacts).toContain("kv-history-migration-plan.json");
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/kv-history/native-plan?namespace=memory_snapshot");
  });

  it("previews native kv_history migration rows through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ kv_history_migration_preview: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T16:30:00Z", stage: "native-kv-history-migration-preview-before-native-write", status: "preview_only", source_namespace: "__kv_history__", native_table: "kv_history", scanned_document_count: 1, preview_row_count: 2, returned_row_count: 2, limit: 50, native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, uses_reserved_kv_namespace: true, rows: [{ id: "kvh-1", namespace: "memory_snapshot", key: "goal", version: 1, value_base64: "InNoaXAi", value_sha256: "h", updated_at: "2026-05-15T15:30:00Z", current: false, source_adapter: "reserved-ledger-kv-namespace" }], artifacts: ["kv-history-migration-preview.json"], labels: ["preview-only"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.nativeKVHistoryMigrationPreview("memory_snapshot", 50);

    expect(result.kv_history_migration_preview.native_kv_history_preview_ready).toBe(true);
    expect(result.kv_history_migration_preview.writes_native_kv_history).toBe(false);
    expect(result.kv_history_migration_preview.rows[0]?.source_adapter).toBe("reserved-ledger-kv-namespace");
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/kv-history/migration-preview?namespace=memory_snapshot&limit=50");
  });

  it("builds native kv_history cutover plans through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ plan: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T17:00:00Z", stage: "kv-history-cutover-plan-before-dual-read-write", status: "plan_only", dry_run: true, requested_by: "operator", reason: "cutover review", source: "native-plan-plus-migration-preview", native_table: "kv_history", current_history_namespace: "__kv_history__", consumes_native_kv_history_plan: true, consumes_migration_preview: true, native_kv_history_plan_ready: true, native_kv_history_preview_ready: true, kv_history_cutover_plan_ready: true, dual_read_plan_ready: true, dual_write_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false, migrates_kv_history: false, dual_read_ready: false, dual_write_ready: false, cutover_ready: false, rollback_ready: false, creates_native_table: false, deletes_reserved_kv_namespace: false, switches_temporal_adapter: false, preview_row_count: 1, returned_preview_row_count: 1, native_kv_history_plan: { native_kv_history_plan_ready: true, native_kv_history_ready: false, writes_native_kv_history: false }, kv_history_migration_preview: { native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, rows: [] }, phases: [], dual_read_plan: { plan_ready: true, ready: false, preferred_source: "native kv_history rows", fallback_source: "__kv_history__", reads_native_kv_history: false, reads_reserved_kv_namespace: true, switches_adapter: false, validation: [], blocked_by: ["dual-read-adapter-not-wired"] }, dual_write_plan: { plan_ready: true, ready: false, primary_target: "__kv_history__", mirror_target: "kv_history", writes_native_kv_history: false, writes_reserved_kv_namespace: false, writes_ledger_kv: false, migration_executor_ready: false, guardrails: [], blocked_by: ["dual-write-cutover-not-enabled"] }, cutover_rollback_plan: { plan_ready: true, ready: false, requires_approval: true, restores_reserved_adapter: true, drops_native_rows: false, deletes_reserved_kv_namespace: false, actions: [], blocked_by: [] }, artifacts: ["kv-history-cutover-plan.json", "kv-history-dual-read-plan.json", "kv-history-dual-write-plan.json"], actions: [], blocked_by: ["dual-read-adapter-not-wired"], labels: ["plan-only"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.kvHistoryCutoverPlan({ namespace: "memory_snapshot", requested_by: "operator", reason: "cutover review", limit: 50, dry_run: true });

    expect(result.plan.kv_history_cutover_plan_ready).toBe(true);
    expect(result.plan.cutover_ready).toBe(false);
    expect(result.plan.writes_native_kv_history).toBe(false);
    expect(result.plan.dual_read_plan.switches_adapter).toBe(false);
    expect(result.plan.dual_write_plan.writes_ledger_kv).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/kv-history/cutover/plan");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
  });

  it("runs native kv_history dual-read parity through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ parity: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory-snapshot", temporal_namespace: "memory_snapshot", generated_at: "2026-05-15T18:00:00Z", at: "2026-05-15T18:00:00Z", stage: "kv-history-dual-read-parity-before-adapter-switch", status: "passed", source: "reserved-temporal-kv-versus-native-row-preview", native_table: "kv_history", current_history_namespace: "__kv_history__", limit: 500, preview_row_count: 2, returned_preview_row_count: 2, temporal_key_count: 2, native_preview_key_count: 2, matched_key_count: 2, mismatch_count: 0, missing_from_native_count: 0, extra_in_native_count: 0, value_mismatch_count: 0, dual_read_parity_check_ready: true, dual_read_parity_ready: true, parity_passed: true, preview_complete: true, reads_temporal_kv: true, reads_native_kv_history: false, reads_native_kv_history_preview: true, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false, kv_history_migration_preview: { native_kv_history_preview_ready: true, writes_native_kv_history: false, migrates_kv_history: false, rows: [] }, mismatches: [], artifacts: ["kv-history-dual-read-parity.json"], actions: [], blocked_by: ["native-kv-history-read-adapter-not-wired"], labels: ["read-only"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.kvHistoryDualReadParity({ namespace: "memory_snapshot", at: "2026-05-15T18:00:00Z", limit: 500 });

    expect(result.parity.dual_read_parity_ready).toBe(true);
    expect(result.parity.parity_passed).toBe(true);
    expect(result.parity.switches_temporal_adapter).toBe(false);
    expect(result.parity.writes_ledger_kv).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/kv-history/dual-read/parity");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
  });


  it("runs kv_history cutover readiness through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ readiness: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory-snapshot", temporal_namespace: "memory_snapshot", generated_at: "2026-05-15T19:00:00Z", at: "2026-05-15T19:00:00Z", stage: "kv-history-cutover-readiness-before-adapter-switch", status: "blocked", dry_run: true, source: "cutover-plan-plus-dual-read-parity", native_table: "kv_history", current_history_namespace: "__kv_history__", cutover_readiness_check_ready: true, cutover_ready: false, native_kv_history_plan_ready: true, native_kv_history_preview_ready: true, dual_read_parity_check_ready: true, dual_read_parity_ready: true, parity_passed: true, preview_complete: true, migration_executor_ready: false, native_read_adapter_ready: false, native_write_path_ready: false, approval_manager_ready: false, rollback_executor_ready: false, audit_proof_link_ready: false, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false, consumes_cutover_plan: true, consumes_dual_read_parity: true, required_gate_count: 7, passed_gate_count: 3, blocked_gate_count: 4, gates: [], cutover_plan: { kv_history_cutover_plan_ready: true, cutover_ready: false, writes_native_kv_history: false }, dual_read_parity: { dual_read_parity_ready: true, parity_passed: true, switches_temporal_adapter: false, writes_ledger_kv: false, writes_native_kv_history: false }, artifacts: ["kv-history-cutover-readiness.json"], actions: [], blocked_by: ["native-kv-history-read-adapter-not-wired"], labels: ["read-only"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.kvHistoryCutoverReadiness({ namespace: "memory_snapshot", at: "2026-05-15T19:00:00Z", requested_by: "operator", reason: "readiness review", limit: 500, dry_run: true });

    expect(result.readiness.cutover_readiness_check_ready).toBe(true);
    expect(result.readiness.cutover_ready).toBe(false);
    expect(result.readiness.parity_passed).toBe(true);
    expect(result.readiness.switches_temporal_adapter).toBe(false);
    expect(result.readiness.writes_ledger_kv).toBe(false);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/kv-history/cutover/readiness");
    expect((spy.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
  });

  it("runs read-only Merkle audit-chain verification through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ ready: true, valid: true, invalid_index: -1, record_count: 2, last_hash: "hash-2", last_seq: 2, checked_at: "2026-05-15T13:00:00Z", recent_records: [{ seq: 2, timestamp: "2026-05-15T13:00:00Z", type: "memory", action: "flush", hash: "hash-2" }] }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.auditVerify(3);

    expect(result.valid).toBe(true);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/audit/verify?limit=3");
  });

  it("reads the KV audit proof-link schema placeholder through pack-owned route", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ links: { pack_id: "yunque.pack.memory-time-travel", namespace: "memory_snapshot", generated_at: "2026-05-15T15:00:00Z", schema_ready: true, linkage_ready: false, native_kv_history_ready: false, merkle_verification_ready: false, source: "schema-placeholder-before-native-kv-history", kv_audit_links: [], required_fields: ["namespace", "key", "audit_hash", "proof_status"] } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const result = await client.auditLinks("memory_snapshot");

    expect(result.links.schema_ready).toBe(true);
    expect(result.links.linkage_ready).toBe(false);
    expect(result.links.kv_audit_links).toEqual([]);
    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/audit/links?namespace=memory_snapshot");
  });
});
