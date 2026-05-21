import { createStateSnapshotClient, StateSnapshotClientError } from "./state-snapshot";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("StateSnapshotClient reads typed state snapshot with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createStateSnapshotClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ goals: [{ id: "g1", title: "ship sdk" }], resources: [{ id: "r1", path: "packages/yunque-client", type: "repo" }], focus: "SDK incremental package", topics: ["sdk"], recent_actions: [{ action: "test", success: true }], capabilities: { total_skills: 12, unresolved_gaps: 1 }, updated_at: "2026-05-12T00:00:00Z" }); } });
  const snapshot = await client.get();
  assertEqual(snapshot.goals[0]?.title, "ship sdk");
  assertEqual(snapshot.resources[0]?.path, "packages/yunque-client");
  assertEqual(snapshot.focus, "SDK incremental package");
  assertEqual(snapshot.capabilities?.unresolved_gaps, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/state");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("StateSnapshotClient uses API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createStateSnapshotClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ goals: [], resources: [] }); } });
  const snapshot = await client.get();
  assertDeepEqual(snapshot.goals, []);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("StateSnapshotClient exposes snapshot nested gateway errors", async () => {
  const client = createStateSnapshotClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "NOT_READY", message: "nested state snapshot failure" } }, { status: 503 }) });
  try { await client.get(); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof StateSnapshotClientError); assertEqual(error.name, "StateClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "NOT_READY", message: "nested state snapshot failure" } }); assertEqual(error.message, "nested state snapshot failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
