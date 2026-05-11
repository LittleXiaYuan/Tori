import { createTaskObserveClient, TaskObserveClientError } from "./task-observe";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskObserveClient lists gaps and stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskObserveClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("stats=true")) return jsonResponse({ total: 2, unresolved: 1 }); return jsonResponse([{ id: "gap-1", gap_type: "skill_missing", resolved: false }]); } });
  const gaps = await client.gaps("skill_missing");
  const stats = await client.gapStats();
  assertEqual(gaps[0]?.id, "gap-1");
  assertEqual(stats.total, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/gaps?type=skill_missing");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/tasks/gaps?stats=true");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TaskObserveClient reads working memory with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskObserveClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ task_id: "task-1", goal: "ship planner", next_action: "resume" }); } });
  const memory = await client.workingMemory("task-1");
  assertEqual(memory.next_action, "resume");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/memory?id=task-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("TaskObserveClient exposes observe nested gateway errors", async () => {
  const client = createTaskObserveClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested task observe failure" } }, { status: 400 }) });
  try { await client.workingMemory(""); throw new Error("expected workingMemory to reject"); } catch (error) { assert(error instanceof TaskObserveClientError); assertEqual(error.name, "TaskContextClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested task observe failure" } }); assertEqual(error.message, "nested task observe failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
