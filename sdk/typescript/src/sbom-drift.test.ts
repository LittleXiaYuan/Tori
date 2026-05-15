import { createSBOMDriftClient, SBOMDriftClientError } from "./sbom-drift";

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

test("SBOMDriftClient reads status and snapshots with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSBOMDriftClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/status")) return jsonResponse({ pack_id: "yunque.pack.sbom-drift", stage: "pack-shell-before-ci", scanner_ready: true, vulnerability_ready: false, snapshot_count: 1, capabilities: [] });
      return jsonResponse({ snapshots: [{ id: "baseline", source: "unit", created_at: "now", component_count: 1, ecosystems: { gomod: 1 } }], count: 1 });
    },
  });

  const status = await client.status();
  const snapshots = await client.snapshots();

  assertEqual(status.pack_id, "yunque.pack.sbom-drift");
  assertEqual(snapshots.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sbom-drift/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/sbom-drift/snapshots");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SBOMDriftClient creates snapshots, reads detail, and diffs current tree", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSBOMDriftClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (init?.method === "POST" && String(url).endsWith("/snapshots")) return jsonResponse({ snapshot: { id: "baseline", source: "manual", created_at: "now", component_count: 0, ecosystems: {}, components: [] }, status: "created" }, { status: 201 });
      if (String(url).includes("/snapshots/")) return jsonResponse({ snapshot: { id: "base line", source: "manual", created_at: "now", component_count: 0, ecosystems: {}, components: [] } });
      return jsonResponse({ diff: { base: { id: "baseline", source: "manual", created_at: "now", component_count: 0, ecosystems: {} }, target: { id: "current", source: "working-tree", created_at: "now", component_count: 1, ecosystems: { npm: 1 } }, added: [], removed: [], changed: [], risk_level: "none" } });
    },
  });

  const created = await client.createSnapshot({ id: "baseline", source: "manual" });
  const detail = await client.snapshot("base line");
  const diff = await client.diff({ base_id: "baseline", target_current: true });

  assertEqual(created.status, "created");
  assertEqual(detail.snapshot.id, "base line");
  assertEqual(diff.diff.risk_level, "none");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sbom-drift/snapshots");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "baseline", source: "manual" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/sbom-drift/snapshots/base%20line");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/sbom-drift/diff");
  assertEqual(calls[2]?.init?.body, JSON.stringify({ base_id: "baseline", target_current: true }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("SBOMDriftClient exports snapshot evidence packs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSBOMDriftClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ pack_id: "yunque.pack.sbom-drift", exported_at: "now", format: "json-sbom-drift-evidence", files: ["snapshot.json"], snapshot: { id: "baseline", source: "unit", created_at: "now", component_count: 0, ecosystems: {}, components: [] } });
    },
  });

  const evidence = await client.evidence("baseline");

  assertEqual(evidence.format, "json-sbom-drift-evidence");
  assertDeepEqual(evidence.files, ["snapshot.json"]);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sbom-drift/evidence/baseline");
});

test("SBOMDriftClient throws SBOMDriftClientError with nested gateway messages", async () => {
  const client = createSBOMDriftClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "pack route is not enabled" }, { status: 404 }),
  });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof SBOMDriftClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "pack route is not enabled" });
    assertEqual(error.message, "pack route is not enabled");
  }

  const nestedClient = createSBOMDriftClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_SNAPSHOT", message: "base_id is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.diff({ base_id: "" });
    throw new Error("expected diff to reject");
  } catch (error) {
    assert(error instanceof SBOMDriftClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "base_id is required");
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
