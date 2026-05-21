import { createTaskMemoryClient, TaskMemoryClientError } from "./task-memory";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskMemoryClient reads working memory with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskMemoryClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ task_id: "task-1", goal: "ship planner", next_action: "resume" }); } });
  const memory = await client.get("task-1");
  assertEqual(memory.next_action, "resume"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/memory?id=task-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TaskMemoryClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskMemoryClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ task_id: "task-2" }); } });
  await client.get("task-2");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("TaskMemoryClient exposes nested memory errors", async () => {
  const client = createTaskMemoryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested task memory failure" } }, { status: 404 }) });
  try { await client.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof TaskMemoryClientError); assertEqual(error.name, "TaskContextClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "nested task memory failure" } }); assertEqual(error.message, "nested task memory failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
