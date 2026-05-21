import { createConnectorConnectClient, ConnectorConnectClientError } from "./connector-connect";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConnectorConnectClient connects with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorConnectClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, status: "connected", user_info: "me@example.com" }); } });
  const result = await client.connect({ connector_id: "gmail", token: "oauth-token" });
  assertEqual(result.ok, true);
  assertEqual(result.status, "connected");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/connectors/connect");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { connector_id: "gmail", token: "oauth-token" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConnectorConnectClient supports api_key credentials and API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorConnectClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, status: "connected" }); } });
  await client.connect({ connector_id: "github", api_key: "ghp_x" });
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { connector_id: "github", api_key: "ghp_x" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("ConnectorConnectClient exposes nested connect errors", async () => {
  const client = createConnectorConnectClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested connector connect failure" } }, { status: 400 }) });
  try { await client.connect({ connector_id: "" }); throw new Error("expected connect to reject"); } catch (error) { assert(error instanceof ConnectorConnectClientError); assertEqual(error.name, "ConnectorsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested connector connect failure" } }); assertEqual(error.message, "nested connector connect failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
