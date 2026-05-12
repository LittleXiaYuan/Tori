import { createFederationPeersClient, FederationPeersClientError } from "./federation-peers";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FederationPeersClient lists peers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationPeersClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ local_id: "local", peers: [{ id: "p1", status: "online" }] }); } });
  const result = await client.list();
  assertEqual(result.local_id, "local");
  assertEqual(result.peers?.[0]?.id, "p1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/peers");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FederationPeersClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationPeersClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ peers: [] }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FederationPeersClient exposes nested peer errors", async () => {
  const client = createFederationPeersClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FED_PEERS", message: "federation peers unavailable" } }, { status: 503 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof FederationPeersClientError); assertEqual(error.name, "FederationClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "FED_PEERS", message: "federation peers unavailable" } }); assertEqual(error.message, "federation peers unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
