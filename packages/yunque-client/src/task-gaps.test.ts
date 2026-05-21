import { createTaskGapsClient, TaskGapsClientError } from "./task-gaps";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskGapsClient lists gaps and stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskGapsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("stats=true")) return jsonResponse({ total: 2, unresolved: 1 }); return jsonResponse([{ id: "gap-1", gap_type: "skill_missing" }]); } });
  const gaps = await client.list("skill_missing"); const stats = await client.stats();
  assertEqual(gaps[0]?.id, "gap-1"); assertEqual(stats.total, 2); assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/gaps?type=skill_missing"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/tasks/gaps?stats=true"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TaskGapsClient resolves gaps with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskGapsClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ resolved: "gap-1" }); } });
  const result = await client.resolve("gap-1");
  assertEqual(result.resolved, "gap-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/gaps/resolve"); assertEqual(calls[0]?.init?.method, "POST"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "gap-1" }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("TaskGapsClient exposes nested gap errors", async () => {
  const client = createTaskGapsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested task gap failure" } }, { status: 400 }) });
  try { await client.resolve(""); throw new Error("expected resolve to reject"); } catch (error) { assert(error instanceof TaskGapsClientError); assertEqual(error.name, "TaskContextClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { message: "nested task gap failure" } }); assertEqual(error.message, "nested task gap failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
