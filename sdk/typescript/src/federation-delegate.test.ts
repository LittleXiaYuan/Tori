import { createFederationDelegateClient, FederationDelegateClientError } from "./federation-delegate";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FederationDelegateClient discovers peers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationDelegateClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ results: [{ peer_id: "p1", agent_id: "a1" }], count: 1 }); } });
  assertEqual((await client.discover({ feature: "browser", intent: "open page", min_tier: "local" })).count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/discover");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { feature: "browser", intent: "open page", min_tier: "local" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FederationDelegateClient delegates work with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFederationDelegateClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "delegated", result: { task_id: "t1" } }); } });
  assertEqual((await client.delegate({ peer_id: "p1", intent: "open page", input: { url: "https://example.test" } })).status, "delegated");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/federation/delegate");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { peer_id: "p1", intent: "open page", input: { url: "https://example.test" } });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FederationDelegateClient exposes nested delegate errors", async () => {
  const client = createFederationDelegateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FEDERATION_DELEGATE", message: "delegate failed" } }, { status: 500 }) });
  try { await client.delegate({ peer_id: "p1" }); throw new Error("expected delegate to reject"); } catch (error) { assert(error instanceof FederationDelegateClientError); assertEqual(error.name, "FederationClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "FEDERATION_DELEGATE", message: "delegate failed" } }); assertEqual(error.message, "delegate failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
