import { createTriggerHistoryClient, TriggerHistoryClientError } from "./trigger-history";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TriggerHistoryClient reads runs and events with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggerHistoryClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("runs")) return jsonResponse({ runs: [{ id: "run-1" }], total: 1 }); return jsonResponse({ events: [{ id: "evt-1" }], total: 1 }); } });
  assertEqual((await client.runs({ triggerId: "v2-1", limit: 5 })).total, 1); assertEqual((await client.events({ triggerId: "v2-1", limit: 7 })).total, 1); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2/runs?trigger_id=v2-1&limit=5"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/triggers/v2/events?trigger_id=v2-1&limit=7"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TriggerHistoryClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggerHistoryClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ runs: [], total: 0 }); } });
  await client.runs();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2/runs"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TriggerHistoryClient exposes nested history errors", async () => {
  const client = createTriggerHistoryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested trigger history failure" } }, { status: 400 }) });
  try { await client.events({ triggerId: "bad" }); throw new Error("expected events to reject"); } catch (error) { assert(error instanceof TriggerHistoryClientError); assertEqual(error.name, "TriggersClientError"); assertEqual(error.status, 400); assertEqual(error.message, "nested trigger history failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
