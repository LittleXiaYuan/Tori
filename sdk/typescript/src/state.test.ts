import { createStateClient, StateClientError } from "./state";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("StateClient reads snapshot goals and focus with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createStateClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); const value = String(url); if (value.endsWith("/goals")) return jsonResponse([{ id: "g1", title: "ship sdk" }]); if (value.endsWith("/focus")) return jsonResponse({ focus: "planner" }); return jsonResponse({ focus: "planner", topics: ["sdk"] }); } });
  const snapshot = await client.snapshot(); const goals = await client.goals(); const focus = await client.focus();
  assertEqual(snapshot.focus, "planner"); assertEqual(goals[0]?.id, "g1"); assertEqual(focus.focus, "planner"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/state"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("StateClient saves and deletes goals with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createStateClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ status: "deleted" }); return jsonResponse({ id: "g1", status: "created" }); } });
  const saved = await client.saveGoal({ title: "ship sdk", priority: 5, task_ids: ["t1"] }); const deleted = await client.deleteGoal("g1");
  assertEqual(saved.status, "created"); assertEqual(deleted.status, "deleted"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/state/goals"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/state/goals?id=g1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { title: "ship sdk", priority: 5, task_ids: ["t1"] });
});

test("StateClient updates focus and topics", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createStateClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  const result = await client.updateFocus("sdk", ["graph", "plugin-api"]);
  assertEqual(result.status, "ok"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/state/focus"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { focus: "sdk", topics: ["graph", "plugin-api"] });
});

test("StateClient manages resources", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createStateClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ status: "tracked" }); if (init?.method === "DELETE") return jsonResponse({ status: "released" }); return jsonResponse([{ id: "r1", path: "sdk/typescript", type: "repo" }]); } });
  const resources = await client.resources(); const tracked = await client.trackResource({ path: "sdk/typescript", type: "repo" }); const released = await client.releaseResource("r1");
  assertEqual(resources[0]?.path, "sdk/typescript"); assertEqual(tracked.status, "tracked"); assertEqual(released.status, "released"); assertEqual(calls[2]?.url, "http://localhost:9090/v1/state/resources?id=r1");
});

test("StateClient throws StateClientError with parsed and text bodies", async () => {
  const jsonClient = createStateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "title required" }, { status: 400 }) });
  try { await jsonClient.saveGoal({ title: "" }); throw new Error("expected saveGoal to reject"); } catch (error) { assert(error instanceof StateClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "title required" }); assertEqual(error.message, "title required"); }
  const nestedClient = createStateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested title required" } }, { status: 400 }) });
  try { await nestedClient.saveGoal({ title: "" }); throw new Error("expected nested saveGoal to reject"); } catch (error) { assert(error instanceof StateClientError); assertEqual(error.status, 400); assertEqual(error.message, "nested title required"); }
  const textClient = createStateClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("state kernel not initialized", { status: 404 }) });
  try { await textClient.snapshot(); throw new Error("expected snapshot to reject"); } catch (error) { assert(error instanceof StateClientError); assertEqual(error.status, 404); assertEqual(error.body, "state kernel not initialized"); assertEqual(error.message, "state kernel not initialized"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
