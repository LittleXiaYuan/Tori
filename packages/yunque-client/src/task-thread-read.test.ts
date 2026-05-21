import { createTaskThreadReadClient, TaskThreadReadClientError } from "./task-thread-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskThreadReadClient lists and gets threads with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskThreadReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("id=task-1")) return jsonResponse({ task_id: "task-1", messages: [{ role: "user", content: "hi" }] }); return jsonResponse({ threads: [{ task_id: "task-1", state: "open" }], total: 1 }); } });
  const list = await client.list("open"); const thread = await client.get("task-1");
  assertEqual(list.total, 1); assertEqual(thread.messages[0]?.content, "hi"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/threads?state=open"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/tasks/threads?id=task-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TaskThreadReadClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskThreadReadClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ threads: [], total: 0 }); } });
  await client.list();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/threads"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("TaskThreadReadClient exposes nested thread read errors", async () => {
  const client = createTaskThreadReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested task thread read failure" } }, { status: 404 }) });
  try { await client.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof TaskThreadReadClientError); assertEqual(error.name, "TaskContextClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "nested task thread read failure" } }); assertEqual(error.message, "nested task thread read failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
