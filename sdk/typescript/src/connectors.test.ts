import { createConnectorsClient, ConnectorsClientError } from "./connectors";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConnectorsClient lists connector catalog with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ connectors: [{ id: "github", name: "GitHub", supported: true, status: "disconnected", action_count: 3 }] }); } });
  const list = await client.list();
  assertEqual(list.connectors[0]?.id, "github"); assertEqual(list.connectors[0]?.action_count, 3); assertEqual(calls[0]?.url, "http://localhost:9090/api/connectors"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConnectorsClient reads detail and connects with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorsClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("detail")) return jsonResponse({ connector: { id: "gmail", name: "Gmail", actions: [{ id: "list_messages" }] }, supported: true, status: "disconnected" }); return jsonResponse({ ok: true, status: "connected", user_info: "me@example.com" }); } });
  const detail = await client.detail("gmail"); const connected = await client.connect({ connector_id: "gmail", token: "oauth-token" });
  assertEqual(detail.connector.actions?.[0]?.id, "list_messages"); assertEqual(connected.status, "connected"); assertEqual(calls[0]?.url, "http://localhost:9090/api/connectors/detail?id=gmail"); assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "ya"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { connector_id: "gmail", token: "oauth-token" });
});

test("ConnectorsClient disconnects and executes actions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/disconnect")) return jsonResponse({ ok: true }); return jsonResponse({ ok: true, result: { message_id: "m1" } }); } });
  const disconnected = await client.disconnect("gmail"); const executed = await client.execute<{ message_id: string }>({ connector_id: "gmail", action_id: "send_email", params: { to: "a@example.com" } });
  assertEqual(disconnected.ok, true); assertEqual(executed.result.message_id, "m1"); assertEqual(calls[0]?.url, "http://localhost:9090/api/connectors/disconnect"); assertEqual(calls[1]?.url, "http://localhost:9090/api/connectors/execute"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { connector_id: "gmail", action_id: "send_email", params: { to: "a@example.com" } });
});

test("ConnectorsClient throws ConnectorsClientError with parsed and text bodies", async () => {
  const jsonClient = createConnectorsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "connector_id required" }, { status: 400 }) });
  try { await jsonClient.connect({ connector_id: "" }); throw new Error("expected connect to reject"); } catch (error) { assert(error instanceof ConnectorsClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "connector_id required" }); assertEqual(error.message, "connector_id required"); }
  const nestedClient = createConnectorsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "connector id is required" } }, { status: 400 }) });
  try { await nestedClient.connect({ connector_id: "" }); throw new Error("expected connect to reject"); } catch (error) { assert(error instanceof ConnectorsClientError); assertEqual(error.status, 400); assertEqual(error.message, "connector id is required"); }
  const textClient = createConnectorsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ConnectorsClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
