import { createTriggersControlClient, TriggersControlClientError } from "./triggers-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TriggersControlClient creates updates and deletes v2 triggers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersControlClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "v2-1", name: "daily", actions: [{}] }, { status: 201 }); if (init?.method === "PUT") return jsonResponse({ id: "v2-1", name: "daily", status: "enabled", actions: [{}] }); return jsonResponse({ deleted: "v2-1" }); } });
  assertEqual((await client.create({ name: "daily", actions: [{}] })).id, "v2-1"); assertEqual((await client.update({ id: "v2-1", name: "daily", status: "enabled", actions: [{}] })).status, "enabled"); assertEqual((await client.delete("v2-1")).deleted, "v2-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2"); assertEqual(calls[1]?.init?.method, "PUT"); assertEqual(calls[2]?.url, "http://localhost:9090/v1/triggers/v2?id=v2-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TriggersControlClient emits v2 events with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersControlClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "emitted", event: "task_completed" }); } });
  assertEqual((await client.emit({ event: "task_completed", text: "done" })).event, "task_completed"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2/emit"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { event: "task_completed", text: "done" }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TriggersControlClient exposes text control errors", async () => {
  const client = createTriggersControlClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("trigger not found", { status: 404 }) });
  try { await client.delete("missing"); throw new Error("expected delete to reject"); } catch (error) { assert(error instanceof TriggersControlClientError); assertEqual(error.name, "TriggersClientError"); assertEqual(error.status, 404); assertEqual(error.body, "trigger not found"); assertEqual(error.message, "trigger not found"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
