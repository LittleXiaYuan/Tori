import { createCognisObserveClient, CognisObserveClientError } from "./cognis-observe";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisObserveClient reads traces stats health verify and alerts with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisObserveClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.includes("traces") || path.includes("/trace")) return jsonResponse({ traces: [{ id: "evt" }], count: 1 }); if (path.includes("stats")) return jsonResponse({ count: 2 }); if (path.includes("health")) return jsonResponse({ healthy: true }); if (path.includes("verify")) return jsonResponse({ ok: true }); return jsonResponse({ alerts: [], count: 0 }); } });
  assertEqual((await client.traces(5)).count, 1);
  assertEqual((await client.trace("doc/id", 2)).count, 1);
  assertEqual((await client.stats() as { count?: number }).count, 2);
  assertEqual((await client.health("doc/id") as { healthy?: boolean }).healthy, true);
  assertEqual((await client.verify() as { ok?: boolean }).ok, true);
  assertEqual((await client.alerts()).count, 0);
  assertEqual((await client.scanAlerts()).count, 0);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/traces?limit=5");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/trace?limit=2");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CognisObserveClient supports API key auth and global health", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisObserveClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ healthy: true }); } });
  await client.health();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/health");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CognisObserveClient exposes nested observe errors", async () => {
  const client = createCognisObserveClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "OBSERVE", message: "cogni observe failed" } }, { status: 503 }) });
  try { await client.traces(); throw new Error("expected traces to reject"); } catch (error) { assert(error instanceof CognisObserveClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "OBSERVE", message: "cogni observe failed" } }); assertEqual(error.message, "cogni observe failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
