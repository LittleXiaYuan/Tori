import { afterEach, describe, expect, it, vi } from "vitest";
import { createMemoryTimeTravelPackClient } from "../memory-time-travel-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("memory-time-travel-pack-client", () => {
  it("reads Memory Time Travel status, snapshots, and detail through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.memory-time-travel", stage: "pack-shell-before-ledger-kv-history", snapshot_store_ready: true, temporal_query_ready: true, ledger_history_ready: false, merkle_verification_ready: false, rollback_writeback_ready: false, retention_plan_ready: true, retention_prune_plan_ready: true, retention_prune_ready: false, kv_audit_link_schema_ready: true, kv_audit_linkage_ready: false, snapshot_count: 1, namespace_count: 1, policy: {}, capabilities: [] }), { status: 200 }))
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

  it("exports JSON evidence packs by snapshot id", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.memory-time-travel", exported_at: "now", format: "json-memory-time-travel-evidence", files: ["snapshot.json", "retention-plan.json", "retention-prune-plan.json", "audit-links.json", "audit-verification.json"], snapshot: { id: "baseline", values: {} }, history: [], retention_plan: { dry_run: true, candidate_count: 0, actions: [] }, retention_prune_plan: { dry_run: true, prune_ready: false, approval_required: false, selected_candidate_count: 0, actions: [] }, kv_audit_link_schema: { schema_ready: true, linkage_ready: false, kv_audit_links: [] }, kv_audit_links: [], audit_verification: { ready: true, valid: true, invalid_index: -1, record_count: 1, checked_at: "now" } }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    const evidence = await client.evidence("baseline");

    expect(evidence.audit_verification?.valid).toBe(true);
    expect(evidence.retention_plan?.dry_run).toBe(true);
    expect(evidence.retention_prune_plan?.prune_ready).toBe(false);
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
