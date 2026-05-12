import { createReverieControlClient, ReverieControlClientError } from "./reverie-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ReverieControlClient updates config with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ config: { enabled: false }, running: false }); } });
  assertEqual((await client.updateConfig({ enabled: false, interval_minutes: 20 })).running, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/reverie/config");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { enabled: false, interval_minutes: 20 });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ReverieControlClient triggers think with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ thought: { id: "t2", category: "event" } }); } });
  assertEqual((await client.think({ event_type: "task_completed", trigger: "demo" })).thought.id, "t2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/reverie/think");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { event_type: "task_completed", trigger: "demo" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ReverieControlClient deletes thoughts", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieControlClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ deleted: true, id: "t1" }); } });
  assertEqual((await client.deleteThought("t1")).deleted, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/reverie/thought?id=t1");
  assertEqual(calls[0]?.init?.method, "DELETE");
});

test("ReverieControlClient exposes nested control errors", async () => {
  const client = createReverieControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "REVERIE_CONTROL", message: "control failed" } }, { status: 500 }) });
  try { await client.think(); throw new Error("expected think to reject"); } catch (error) { assert(error instanceof ReverieControlClientError); assertEqual(error.name, "ReverieClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "REVERIE_CONTROL", message: "control failed" } }); assertEqual(error.message, "control failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
