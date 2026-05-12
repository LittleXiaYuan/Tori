import { createFederationStatsClient, FederationStatsClientError } from "./federation-stats";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FederationStatsClient reads federation and bridge stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationStatsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return String(url).includes("bridge") ? jsonResponse({ configured: true }) : jsonResponse({ peers_online: 2 }); } });
  assertEqual((await client.stats()).peers_online, 2);
  assertEqual((await client.bridge()).configured, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/stats");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/federation/bridge/stats");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FederationStatsClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationStatsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ configured: false }); } });
  await client.bridge();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FederationStatsClient exposes nested stats errors", async () => {
  const client = createFederationStatsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FED_STATS", message: "federation stats unavailable" } }, { status: 503 }) });
  try { await client.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof FederationStatsClientError); assertEqual(error.name, "FederationClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "FED_STATS", message: "federation stats unavailable" } }); assertEqual(error.message, "federation stats unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
