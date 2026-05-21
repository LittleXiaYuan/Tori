import { createRuntimeQueueClient, RuntimeQueueClientError } from "./runtime-queue";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("RuntimeQueueClient reads queue overview with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeQueueClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ queues: { "session-1": 2 } }); } });
  const overview = await client.overview();
  assertEqual(overview.queues?.["session-1"], 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sessions/queue");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("RuntimeQueueClient reads session queues and cancels tasks with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeQueueClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/cancel")) return jsonResponse({ cancelled: true }); return jsonResponse({ session_id: "session/1", tasks: [{ task_id: "task-1", status: "queued" }] }); } });
  const queue = await client.session("session/1");
  const cancelled = await client.cancel("session/1", "task-1");
  assertEqual(queue.tasks[0]?.status, "queued");
  assertEqual(cancelled.cancelled, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sessions/queue?id=session%2F1");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/sessions/queue/cancel");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { session_id: "session/1", task_id: "task-1" });
  assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "ya");
});

test("RuntimeQueueClient exposes nested runtime queue errors", async () => {
  const client = createRuntimeQueueClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested runtime queue failure" } }, { status: 400 }) });
  try { await client.cancel("", "task-1"); throw new Error("expected cancel to reject"); } catch (error) { assert(error instanceof RuntimeQueueClientError); assertEqual(error.name, "RuntimeClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested runtime queue failure" } }); assertEqual(error.message, "nested runtime queue failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
