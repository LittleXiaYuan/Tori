import { createRuntimeQueueReadClient, RuntimeQueueReadClientError } from "./runtime-queue-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("RuntimeQueueReadClient reads queue overview with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeQueueReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ queues: { "session-1": 2 } }); } });
  const overview = await client.overview();
  assertEqual(overview.queues?.["session-1"], 2); assertEqual(calls[0]?.url, "http://localhost:9090/v1/sessions/queue"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("RuntimeQueueReadClient reads session queue with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeQueueReadClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ session_id: "session/1", tasks: [{ task_id: "task-1", status: "queued" }] }); } });
  const queue = await client.session("session/1");
  assertEqual(queue.tasks[0]?.status, "queued"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/sessions/queue?id=session%2F1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("RuntimeQueueReadClient exposes nested queue read errors", async () => {
  const client = createRuntimeQueueReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested runtime queue read failure" } }, { status: 400 }) });
  try { await client.session(""); throw new Error("expected session to reject"); } catch (error) { assert(error instanceof RuntimeQueueReadClientError); assertEqual(error.name, "RuntimeClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested runtime queue read failure" } }); assertEqual(error.message, "nested runtime queue read failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
