import { createDispatchQueueClient, DispatchQueueClientError } from "./dispatch-queue";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DispatchQueueClient reads queue with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchQueueClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ message: "dispatch queue", queues: { coding: 2 } }); } });
  const result = await client.queue();
  assertEqual(result.message, "dispatch queue");
  assertDeepEqual(result.queues, { coding: 2 });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/dispatch/queue");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("DispatchQueueClient reads queue with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchQueueClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ message: "ok" }); } });
  const result = await client.queue();
  assertEqual(result.message, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/dispatch/queue");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("DispatchQueueClient exposes dispatch-queue nested gateway errors", async () => {
  const client = createDispatchQueueClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DISPATCH_QUEUE", message: "nested dispatch queue failure" } }, { status: 503 }) });
  try { await client.queue(); throw new Error("expected queue to reject"); } catch (error) { assert(error instanceof DispatchQueueClientError); assertEqual(error.name, "DispatchClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "DISPATCH_QUEUE", message: "nested dispatch queue failure" } }); assertEqual(error.message, "nested dispatch queue failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
