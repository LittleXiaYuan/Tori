import { createFederationCapabilitiesClient, FederationCapabilitiesClientError } from "./federation-capabilities";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FederationCapabilitiesClient reads updates and broadcasts capabilities with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationCapabilitiesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/broadcast")) return jsonResponse({ status: "broadcasted" }); if (init?.method === "POST") return jsonResponse({ status: "updated" }); return jsonResponse({ local: { agent_id: "a1" }, peers: [] }); } });
  assertEqual(((await client.get()).local as { agent_id?: string }).agent_id, "a1");
  assertEqual((await client.update({ agent_id: "a1", features: ["chat"] })).status, "updated");
  assertEqual((await client.broadcast()).status, "broadcasted");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/capabilities");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/federation/capabilities");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/federation/broadcast");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { agent_id: "a1", features: ["chat"] });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FederationCapabilitiesClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationCapabilitiesClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "updated" }); } });
  await client.update({ agent_id: "a1" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FederationCapabilitiesClient exposes nested capability errors", async () => {
  const client = createFederationCapabilitiesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FED_CAP", message: "capability update denied" } }, { status: 403 }) });
  try { await client.update({ agent_id: "a1" }); throw new Error("expected update to reject"); } catch (error) { assert(error instanceof FederationCapabilitiesClientError); assertEqual(error.name, "FederationClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "FED_CAP", message: "capability update denied" } }); assertEqual(error.message, "capability update denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
