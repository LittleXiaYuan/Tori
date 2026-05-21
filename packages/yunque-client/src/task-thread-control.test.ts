import { createTaskThreadControlClient, TaskThreadControlClientError } from "./task-thread-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskThreadControlClient posts messages with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskThreadControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "posted", task_id: "task-1" }, { status: 201 }); } });
  const result = await client.postMessage("task-1", "请继续", { channel_type: "feishu", channel_id: "chat-1" });
  assertEqual(result.status, "posted"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/threads"); assertEqual(calls[0]?.init?.method, "POST"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { task_id: "task-1", content: "请继续", channel: { channel_type: "feishu", channel_id: "chat-1" } }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TaskThreadControlClient updates state and omits channel with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskThreadControlClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "updated", task_id: "task-1", state: "paused" }); } });
  await client.postMessage("task-1", "hi"); const updated = await client.updateState("task-1", "paused");
  assertEqual(updated.state, "paused"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { task_id: "task-1", content: "hi" }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { task_id: "task-1", state: "paused" }); assertEqual(calls[1]?.init?.method, "PUT"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("TaskThreadControlClient exposes nested thread control errors", async () => {
  const client = createTaskThreadControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested task thread control failure" } }, { status: 400 }) });
  try { await client.updateState("", "paused"); throw new Error("expected updateState to reject"); } catch (error) { assert(error instanceof TaskThreadControlClientError); assertEqual(error.name, "TaskContextClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { message: "nested task thread control failure" } }); assertEqual(error.message, "nested task thread control failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
