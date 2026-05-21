import { createCognisFederationClient, CognisFederationClientError } from "./cognis-federation";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisFederationClient reads status, peers and economics with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisFederationClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/peers")) return jsonResponse({ peers: ["a"] }); if (path.endsWith("/economics")) return jsonResponse({ balance: 3 }); return jsonResponse({ enabled: true }); } });
  assertEqual((await client.status() as { enabled?: boolean }).enabled, true);
  assertDeepEqual((await client.peers() as { peers?: string[] }).peers, ["a"]);
  assertEqual((await client.economics() as { balance?: number }).balance, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/federation");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/federation/peers");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cognis/economics");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CognisFederationClient discovers peers and toggles exposure with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisFederationClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.discover({ region: "cn" });
  await client.discover();
  await client.expose("doc/id");
  await client.unexpose("doc/id");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/federation/discover");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { region: "cn" });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), {});
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/expose");
  assertEqual(calls[3]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/unexpose");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CognisFederationClient exposes nested federation errors", async () => {
  const client = createCognisFederationClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FEDERATION", message: "federation failed" } }, { status: 502 }) });
  try { await client.discover(); throw new Error("expected discover to reject"); } catch (error) { assert(error instanceof CognisFederationClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 502); assertDeepEqual(error.body, { error: { code: "FEDERATION", message: "federation failed" } }); assertEqual(error.message, "federation failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
