import { createTriggersClient, TriggersClientError } from "./triggers";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TriggersClient manages legacy triggers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); const value = String(url); if (value.includes("emit")) return jsonResponse({ status: "emitted", event: "task_completed" }); if (init?.method === "POST") return jsonResponse({ id: "trg-1", name: "legacy" }, { status: 201 }); if (init?.method === "DELETE") return jsonResponse({ deleted: "trg-1" }); if (value.includes("id=")) return jsonResponse({ id: "trg-1", name: "legacy" }); return jsonResponse({ triggers: [{ id: "trg-1", name: "legacy" }], total: 1 }); } });
  assertEqual((await client.listLegacy()).total, 1); assertEqual((await client.getLegacy("trg-1")).name, "legacy"); assertEqual((await client.createLegacy({ name: "legacy" })).id, "trg-1"); assertEqual((await client.emitLegacy({ event: "task_completed" })).status, "emitted"); assertEqual((await client.deleteLegacy("trg-1")).deleted, "trg-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TriggersClient lists creates updates and deletes v2 triggers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "v2-1", name: "daily", actions: [{}] }, { status: 201 }); if (init?.method === "PUT") return jsonResponse({ id: "v2-1", name: "daily", status: "enabled", actions: [{}] }); if (init?.method === "DELETE") return jsonResponse({ deleted: "v2-1" }); if (new URL(String(url)).searchParams.get("id")) return jsonResponse({ id: "v2-1", name: "daily" }); return jsonResponse({ triggers: [{ id: "v2-1", name: "daily" }], total: 1 }); } });
  assertEqual((await client.list({ tenantId: "default", type: "event", status: "enabled" })).total, 1); assertEqual((await client.get("v2-1")).name, "daily"); assertEqual((await client.create({ name: "daily", actions: [{}] })).id, "v2-1"); assertEqual((await client.update({ id: "v2-1", name: "daily", status: "enabled", actions: [{}] })).status, "enabled"); assertEqual((await client.delete("v2-1")).deleted, "v2-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2?tenant_id=default&type=event&status=enabled"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TriggersClient emits v2 events and reads runs/events", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("emit")) return jsonResponse({ status: "emitted", event: "task_completed" }); if (String(url).includes("runs")) return jsonResponse({ runs: [{ id: "run-1" }], total: 1 }); return jsonResponse({ events: [{ id: "evt-1" }], total: 1 }); } });
  assertEqual((await client.emit({ event: "task_completed", text: "done" })).event, "task_completed"); assertEqual((await client.runs({ triggerId: "v2-1", limit: 5 })).total, 1); assertEqual((await client.events({ triggerId: "v2-1", limit: 7 })).total, 1); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { event: "task_completed", text: "done" }); assertEqual(calls[1]?.url, "http://localhost:9090/v1/triggers/v2/runs?trigger_id=v2-1&limit=5"); assertEqual(calls[2]?.url, "http://localhost:9090/v1/triggers/v2/events?trigger_id=v2-1&limit=7");
});

test("TriggersClient throws TriggersClientError with parsed and text bodies", async () => {
  const jsonClient = createTriggersClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "name is required" }, { status: 400 }) });
  try { await jsonClient.create({ name: "", actions: [] }); throw new Error("expected create to reject"); } catch (error) { assert(error instanceof TriggersClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "name is required" }); assertEqual(error.message, "name is required"); }
  const textClient = createTriggersClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("trigger not found", { status: 404 }) });
  try { await textClient.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof TriggersClientError); assertEqual(error.status, 404); assertEqual(error.body, "trigger not found"); assertEqual(error.message, "trigger not found"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);

