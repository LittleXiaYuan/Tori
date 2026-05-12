import { createFederationObserveClient, FederationObserveClientError } from "./federation-observe";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FederationObserveClient reads peers and stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationObserveClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/peers")) return jsonResponse({ local_id: "local", peers: [{ id: "p1" }] }); return jsonResponse({ peers: 1, messages: 2 }); } });
  assertEqual((await client.peers()).local_id, "local");
  assertEqual((await client.stats()).messages, 2);
  assertDeepEqual(calls.map((call) => call.url), ["http://localhost:9090/v1/federation/peers", "http://localhost:9090/v1/federation/stats"]);
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FederationObserveClient reads capabilities and bridge stats with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationObserveClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/capabilities")) return jsonResponse({ local: { agent_id: "agent-a" }, peers: [] }); return jsonResponse({ configured: true, peers: 2 }); } });
  assertEqual(((await client.capabilities()).local as { agent_id?: string }).agent_id, "agent-a");
  assertEqual((await client.bridgeStats()).configured, true);
  assertDeepEqual(calls.map((call) => call.url), ["http://localhost:9090/v1/federation/capabilities", "http://localhost:9090/v1/federation/bridge/stats"]);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FederationObserveClient exposes nested observe errors", async () => {
  const client = createFederationObserveClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FEDERATION_OBSERVE", message: "observe failed" } }, { status: 500 }) });
  try { await client.capabilities(); throw new Error("expected capabilities to reject"); } catch (error) { assert(error instanceof FederationObserveClientError); assertEqual(error.name, "FederationClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "FEDERATION_OBSERVE", message: "observe failed" } }); assertEqual(error.message, "observe failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
