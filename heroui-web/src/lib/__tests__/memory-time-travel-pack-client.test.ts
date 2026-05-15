import { afterEach, describe, expect, it, vi } from "vitest";
import { createMemoryTimeTravelPackClient } from "../memory-time-travel-pack-client";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("memory-time-travel-pack-client", () => {
  it("reads Memory Time Travel status, snapshots, and detail through pack-owned routes", async () => {
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.memory-time-travel", stage: "pack-shell-before-ledger-kv-history", snapshot_store_ready: true, temporal_query_ready: true, ledger_history_ready: false, merkle_verification_ready: false, rollback_writeback_ready: false, snapshot_count: 1, namespace_count: 1, policy: {}, capabilities: [] }), { status: 200 }))
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
      .mockResolvedValueOnce(new Response(JSON.stringify({ pack_id: "yunque.pack.memory-time-travel", exported_at: "now", format: "json-memory-time-travel-evidence", files: ["snapshot.json"], snapshot: { id: "baseline", values: {} }, history: [] }), { status: 200 }));

    const client = createMemoryTimeTravelPackClient();
    await client.evidence("baseline");

    expect(spy.mock.calls[0]?.[0]).toBe("/v1/memory-time-travel/evidence/baseline");
  });
});
