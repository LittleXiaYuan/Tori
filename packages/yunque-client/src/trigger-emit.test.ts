import { createTriggerEmitClient, TriggerEmitClientError } from "./trigger-emit";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TriggerEmitClient emits v2 events with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggerEmitClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "emitted", event: "task_completed" }); } });
  const result = await client.emit({ event: "task_completed", text: "done" });
  assertEqual(result.event, "task_completed"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2/emit"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { event: "task_completed", text: "done" }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TriggerEmitClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggerEmitClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "emitted", event: "task_completed" }); } });
  await client.emit({ event: "task_completed" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TriggerEmitClient exposes nested emit errors", async () => {
  const client = createTriggerEmitClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested trigger emit failure" } }, { status: 400 }) });
  try { await client.emit({ event: "" }); throw new Error("expected emit to reject"); } catch (error) { assert(error instanceof TriggerEmitClientError); assertEqual(error.name, "TriggersClientError"); assertEqual(error.status, 400); assertEqual(error.message, "nested trigger emit failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
