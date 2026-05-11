import { createConnectorCatalogClient, ConnectorCatalogClientError } from "./connector-catalog";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConnectorCatalogClient lists connector catalog with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorCatalogClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ connectors: [{ id: "github", name: "GitHub", supported: true, status: "disconnected", action_count: 3 }] }); } });
  const list = await client.list();
  assertEqual(list.connectors[0]?.id, "github");
  assertEqual(list.connectors[0]?.action_count, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/connectors");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConnectorCatalogClient reads connector detail with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorCatalogClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ connector: { id: "gmail", name: "Gmail", actions: [{ id: "list_messages" }] }, supported: true, status: "disconnected" }); } });
  const detail = await client.detail("gmail");
  assertEqual(detail.connector.actions?.[0]?.id, "list_messages");
  assertEqual(detail.status, "disconnected");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/connectors/detail?id=gmail");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("ConnectorCatalogClient exposes catalog nested gateway errors", async () => {
  const client = createConnectorCatalogClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested connector catalog failure" } }, { status: 400 }) });
  try { await client.detail(""); throw new Error("expected detail to reject"); } catch (error) { assert(error instanceof ConnectorCatalogClientError); assertEqual(error.name, "ConnectorsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested connector catalog failure" } }); assertEqual(error.message, "nested connector catalog failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
