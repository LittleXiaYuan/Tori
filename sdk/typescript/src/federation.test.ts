import { createFederationClient, FederationClientError } from "./federation";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FederationClient reads legacy peers and stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/peers")) return jsonResponse({ local_id: "agent-local", peers: [{ id: "peer-a" }] }); return jsonResponse({ peers: 1, messages: 2 }); } });
  const peers = await client.peers(); const stats = await client.stats();
  assertEqual(peers.local_id, "agent-local"); assertEqual(peers.peers?.[0]?.id, "peer-a"); assertEqual((stats as { messages?: number }).messages, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/peers"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/federation/stats");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("FederationClient reads and updates OPP capabilities with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ status: "updated" }); return jsonResponse({ local: { agent_id: "agent-a", features: ["chat"] }, peers: [] }); } });
  const caps = await client.capabilities(); const updated = await client.updateCapabilities({ agent_id: "agent-a", features: ["chat", "planner"] });
  assertEqual((caps.local as { agent_id?: string }).agent_id, "agent-a"); assertEqual(updated.status, "updated");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/capabilities"); assertEqual(calls[1]?.init?.method, "POST");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ agent_id: "agent-a", features: ["chat", "planner"] })); assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "key-123");
});

test("FederationClient discovers and delegates work", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/discover")) return jsonResponse({ results: [{ peer_id: "p1", agent_id: "a1", features: ["browser"] }], count: 1 }); return jsonResponse({ status: "delegated", result: { task_id: "t1" } }); } });
  const found = await client.discover({ feature: "browser", intent: "open page", min_tier: "local", features: ["browser"] });
  const delegated = await client.delegate({ peer_id: "p1", intent: "open page", input: { url: "https://example.test" } });
  assertEqual(found.count, 1); assertEqual(found.results[0]?.peer_id, "p1"); assertEqual(delegated.status, "delegated");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/discover"); assertEqual(calls[0]?.init?.body, JSON.stringify({ feature: "browser", intent: "open page", min_tier: "local", features: ["browser"] }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/federation/delegate");
});

test("FederationClient reads bridge stats and broadcasts capabilities", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("bridge/stats")) return jsonResponse({ configured: true, peers: 2 }); return jsonResponse({ status: "broadcasted" }); } });
  const bridge = await client.bridgeStats(); const broadcast = await client.broadcast();
  assertEqual(bridge.configured, true); assertEqual((bridge as { peers?: number }).peers, 2); assertEqual(broadcast.status, "broadcasted");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/bridge/stats"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/federation/broadcast"); assertEqual(calls[1]?.init?.method, "POST");
});

test("FederationClient throws FederationClientError with parsed and text bodies", async () => {
  const jsonClient = createFederationClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "federation bridge not configured" }, { status: 500 }) });
  try { await jsonClient.capabilities(); throw new Error("expected capabilities to reject"); } catch (error) { assert(error instanceof FederationClientError); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: "federation bridge not configured" }); assertEqual(error.message, "federation bridge not configured"); }
  const textClient = createFederationClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST only", { status: 405 }) });
  try { await textClient.discover({ feature: "browser" }); throw new Error("expected discover to reject"); } catch (error) { assert(error instanceof FederationClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST only"); assertEqual(error.message, "POST only"); }
});
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
