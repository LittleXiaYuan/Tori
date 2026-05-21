import { createGoalStateClient, GoalStateClientError } from "./goal-state";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("GoalStateClient lists goals with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGoalStateClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse([{ id: "g1", title: "ship sdk" }]); } });
  const goals = await client.list();
  assertEqual(goals[0]?.id, "g1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/state/goals");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("GoalStateClient saves and deletes goals with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGoalStateClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ status: "deleted" }); return jsonResponse({ id: "g1", status: "created" }); } });
  const saved = await client.save({ title: "ship sdk", priority: 5, task_ids: ["t1"] });
  const deleted = await client.delete("g1");
  assertEqual(saved.status, "created");
  assertEqual(deleted.status, "deleted");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/state/goals");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/state/goals?id=g1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { title: "ship sdk", priority: 5, task_ids: ["t1"] });
});

test("GoalStateClient exposes goal-state nested gateway errors", async () => {
  const client = createGoalStateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested goal failure" } }, { status: 400 }) });
  try { await client.save({ title: "" }); throw new Error("expected save to reject"); } catch (error) { assert(error instanceof GoalStateClientError); assertEqual(error.name, "StateClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested goal failure" } }); assertEqual(error.message, "nested goal failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
