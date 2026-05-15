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
      if (String(url).endsWith("/status")) return jsonResponse({ pack_id: "yunque.pack.memory-time-travel", stage: "pack-shell-before-ledger-kv-history", snapshot_store_ready: true, temporal_query_ready: true, ledger_history_ready: false, merkle_verification_ready: false, rollback_writeback_ready: false, snapshot_count: 1, namespace_count: 1, policy: {}, capabilities: [] });
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

test("MemoryTimeTravelClient reads detail and exports evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryTimeTravelClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("/snapshots/")) return jsonResponse({ snapshot });
      return jsonResponse({ pack_id: "yunque.pack.memory-time-travel", exported_at: "now", format: "json-memory-time-travel-evidence", files: ["snapshot.json"], snapshot, history: [] });
    },
  });

  const detail = await client.snapshot("baseline");
  const evidence = await client.evidence("baseline");

  assertEqual(detail.snapshot.id, "baseline");
  assertEqual(evidence.format, "json-memory-time-travel-evidence");
  assertDeepEqual(evidence.files, ["snapshot.json"]);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory-time-travel/snapshots/baseline");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/memory-time-travel/evidence/baseline");
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
