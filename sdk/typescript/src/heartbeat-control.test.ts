import { createHeartbeatControlClient, HeartbeatControlClientError } from "./heartbeat-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("HeartbeatControlClient updates enabled state with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createHeartbeatControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.update({ enabled: true, interval_minutes: 30 })).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/heartbeat");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { enabled: true, interval_minutes: 30 });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("HeartbeatControlClient triggers heartbeat with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createHeartbeatControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ id: "hb1", status: "ok" }); } });
  assertEqual((await client.trigger()).id, "hb1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/heartbeat/trigger");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("HeartbeatControlClient exposes nested control errors", async () => {
  const client = createHeartbeatControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "HEARTBEAT_CONTROL", message: "control failed" } }, { status: 500 }) });
  try { await client.trigger(); throw new Error("expected trigger to reject"); } catch (error) { assert(error instanceof HeartbeatControlClientError); assertEqual(error.name, "HeartbeatClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "HEARTBEAT_CONTROL", message: "control failed" } }); assertEqual(error.message, "control failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
