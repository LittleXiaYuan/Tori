import { createFederationControlClient, FederationControlClientError } from "./federation-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FederationControlClient updates capabilities with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "updated" }); } });
  assertEqual((await client.updateCapabilities({ agent_id: "agent-a", features: ["chat", "planner"] })).status, "updated");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/capabilities");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { agent_id: "agent-a", features: ["chat", "planner"] });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FederationControlClient broadcasts capabilities with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "broadcasted" }); } });
  assertEqual((await client.broadcast()).status, "broadcasted");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/broadcast");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FederationControlClient exposes nested control errors", async () => {
  const client = createFederationControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FEDERATION_CONTROL", message: "control failed" } }, { status: 500 }) });
  try { await client.broadcast(); throw new Error("expected broadcast to reject"); } catch (error) { assert(error instanceof FederationControlClientError); assertEqual(error.name, "FederationClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "FEDERATION_CONTROL", message: "control failed" } }); assertEqual(error.message, "control failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
