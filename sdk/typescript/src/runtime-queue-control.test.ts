import { createRuntimeQueueControlClient, RuntimeQueueControlClientError } from "./runtime-queue-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("RuntimeQueueControlClient cancels tasks with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeQueueControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ cancelled: true }); } });
  const cancelled = await client.cancel("session/1", "task-1");
  assertEqual(cancelled.cancelled, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/sessions/queue/cancel"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { session_id: "session/1", task_id: "task-1" });
});

test("RuntimeQueueControlClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeQueueControlClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ cancelled: false }); } });
  await client.cancel("s", "t");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("RuntimeQueueControlClient exposes nested queue control errors", async () => {
  const client = createRuntimeQueueControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested runtime queue control failure" } }, { status: 400 }) });
  try { await client.cancel("", "task-1"); throw new Error("expected cancel to reject"); } catch (error) { assert(error instanceof RuntimeQueueControlClientError); assertEqual(error.name, "RuntimeClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested runtime queue control failure" } }); assertEqual(error.message, "nested runtime queue control failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
