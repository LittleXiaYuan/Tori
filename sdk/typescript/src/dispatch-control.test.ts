import { createDispatchControlClient, DispatchControlClientError } from "./dispatch-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DispatchControlClient removes workers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createDispatchControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ status: "removed" }); } });
  const removed = await client.removeWorker("w1");
  assertEqual(removed.status, "removed");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers/remove");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(calls[0]?.body, { id: "w1" });
});

test("DispatchControlClient enqueues tasks with API key", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createDispatchControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ task_id: "t1", status: "enqueued" }); } });
  const enqueued = await client.enqueue({ task_id: "t1", capabilities: ["coding", "testing"], priority: 10 });
  assertEqual(enqueued.status, "enqueued");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/dispatch/enqueue");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(calls[0]?.body, { task_id: "t1", capabilities: ["coding", "testing"], priority: 10 });
});

test("DispatchControlClient exposes nested control errors", async () => {
  const client = createDispatchControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DISPATCH_CONTROL", message: "task id required" } }, { status: 400 }) });
  try { await client.enqueue({ task_id: "" }); throw new Error("expected enqueue to reject"); } catch (error) { assert(error instanceof DispatchControlClientError); assertEqual(error.name, "DispatchClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "DISPATCH_CONTROL", message: "task id required" } }); assertEqual(error.message, "task id required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
