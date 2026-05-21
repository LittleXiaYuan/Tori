import { createTaskThreadsClient, TaskThreadsClientError } from "./task-threads";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskThreadsClient lists and gets threads with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskThreadsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("id=task-1")) return jsonResponse({ task_id: "task-1", info: { state: "open" }, messages: [{ role: "user", content: "hi" }] }); return jsonResponse({ threads: [{ task_id: "task-1", state: "open" }], total: 1 }); } });
  const list = await client.list("open");
  const thread = await client.get("task-1");
  assertEqual(list.total, 1);
  assertEqual(thread.messages[0]?.content, "hi");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/threads?state=open");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/tasks/threads?id=task-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TaskThreadsClient posts messages and updates state with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskThreadsClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "PUT") return jsonResponse({ status: "updated", task_id: "task-1", state: "paused" }); return jsonResponse({ status: "posted", task_id: "task-1" }, { status: 201 }); } });
  const posted = await client.postMessage("task-1", "请继续", { channel_type: "feishu", channel_id: "chat-1", user_id: "u1" });
  const updated = await client.updateState("task-1", "paused");
  assertEqual(posted.status, "posted");
  assertEqual(updated.state, "paused");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/threads");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { task_id: "task-1", content: "请继续", channel: { channel_type: "feishu", channel_id: "chat-1", user_id: "u1" } });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { task_id: "task-1", state: "paused" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("TaskThreadsClient omits channel when posting direct messages", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskThreadsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "posted", task_id: "task-1" }); } });
  await client.postMessage("task-1", "hi");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { task_id: "task-1", content: "hi" });
});

test("TaskThreadsClient exposes thread nested gateway errors", async () => {
  const client = createTaskThreadsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested task thread failure" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof TaskThreadsClientError); assertEqual(error.name, "TaskContextClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested task thread failure" } }); assertEqual(error.message, "nested task thread failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
