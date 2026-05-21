import { createReverieObserveClient, ReverieObserveClientError } from "./reverie-observe";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ReverieObserveClient reads journal with bearer token and filters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieObserveClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ thoughts: [{ id: "t1" }], total: 1, limit: 10, offset: 0 }); } });
  assertEqual((await client.journal({ category: "task", delivered: false, limit: 10 })).total, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/reverie/journal?category=task&delivered=false&limit=10");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ReverieObserveClient reads stats config actions and targets with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieObserveClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); const text = String(url); if (text.endsWith("/stats")) return jsonResponse({ total: 2 }); if (text.endsWith("/config")) return jsonResponse({ config: { enabled: true }, running: true }); if (text.endsWith("/actions")) return jsonResponse({ actions: [{ id: "a1" }], total: 1 }); return jsonResponse({ targets: [{ channel: "feishu" }], count: 1 }); } });
  assertEqual((await client.stats()).total, 2);
  assertEqual((await client.config()).running, true);
  assertEqual((await client.actions()).total, 1);
  assertEqual((await client.targets()).count, 1);
  assertDeepEqual(calls.map((call) => call.url), ["http://localhost:9090/v1/reverie/stats", "http://localhost:9090/v1/reverie/config", "http://localhost:9090/v1/reverie/actions", "http://localhost:9090/v1/reverie/targets"]);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ReverieObserveClient exposes nested observe errors", async () => {
  const client = createReverieObserveClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "REVERIE_OBSERVE", message: "observe failed" } }, { status: 500 }) });
  try { await client.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof ReverieObserveClientError); assertEqual(error.name, "ReverieClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "REVERIE_OBSERVE", message: "observe failed" } }); assertEqual(error.message, "observe failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
