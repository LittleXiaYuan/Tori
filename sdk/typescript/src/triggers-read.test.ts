import { createTriggersReadClient, TriggersReadClientError } from "./triggers-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TriggersReadClient lists and gets v2 triggers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersReadClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (new URL(String(url)).searchParams.get("id")) return jsonResponse({ id: "v2-1", name: "daily" }); return jsonResponse({ triggers: [{ id: "v2-1", name: "daily" }], total: 1 }); } });
  assertEqual((await client.list({ tenantId: "default", type: "event", status: "enabled" })).total, 1); assertEqual((await client.get("v2-1")).name, "daily"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2?tenant_id=default&type=event&status=enabled"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/triggers/v2?id=v2-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TriggersReadClient reads runs and events with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersReadClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("runs")) return jsonResponse({ runs: [{ id: "run-1" }], total: 1 }); return jsonResponse({ events: [{ id: "evt-1" }], total: 1 }); } });
  assertEqual((await client.runs({ triggerId: "v2-1", limit: 5 })).total, 1); assertEqual((await client.events({ triggerId: "v2-1", limit: 7 })).total, 1); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2/runs?trigger_id=v2-1&limit=5"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/triggers/v2/events?trigger_id=v2-1&limit=7"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TriggersReadClient exposes nested read errors", async () => {
  const client = createTriggersReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "trigger id is required" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof TriggersReadClientError); assertEqual(error.name, "TriggersClientError"); assertEqual(error.status, 400); assertEqual(error.message, "trigger id is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
