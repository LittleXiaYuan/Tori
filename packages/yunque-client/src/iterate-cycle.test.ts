import { createIterateCycleClient, IterateCycleClientError } from "./iterate-cycle";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("IterateCycleClient reads status with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIterateCycleClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ enabled: true, pending_proposals: 2 }); } });
  assertEqual((await client.status()).enabled, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/iterate/status");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("IterateCycleClient triggers a cycle with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIterateCycleClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok", cycle: { rounds: 1 } }); } });
  assertEqual((await client.trigger()).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/iterate/trigger");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
});

test("IterateCycleClient exposes nested cycle errors", async () => {
  const client = createIterateCycleClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "ITERATE_CYCLE", message: "cycle failed" } }, { status: 500 }) });
  try { await client.trigger(); throw new Error("expected trigger to reject"); } catch (error) { assert(error instanceof IterateCycleClientError); assertEqual(error.name, "IterateClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "ITERATE_CYCLE", message: "cycle failed" } }); assertEqual(error.message, "cycle failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
