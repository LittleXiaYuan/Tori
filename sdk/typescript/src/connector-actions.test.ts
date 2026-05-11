import { createConnectorActionsClient, ConnectorActionsClientError } from "./connector-actions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConnectorActionsClient executes connector actions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorActionsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, result: { issue_count: 2 } }); } });
  const result = await client.execute<{ issue_count: number }>({ connector_id: "github", action_id: "list_issues", params: { state: "open" } });
  assertEqual(result.ok, true);
  assertEqual(result.result.issue_count, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/connectors/execute");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { connector_id: "github", action_id: "list_issues", params: { state: "open" } });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConnectorActionsClient fills empty params with API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConnectorActionsClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, result: { sent: true } }); } });
  const result = await client.execute<{ sent: boolean }>({ connector_id: "gmail", action_id: "send_email" });
  assertEqual(result.result.sent, true);
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { connector_id: "gmail", action_id: "send_email", params: {} });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("ConnectorActionsClient exposes action nested gateway errors", async () => {
  const client = createConnectorActionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested connector action failure" } }, { status: 400 }) });
  try { await client.execute({ connector_id: "", action_id: "" }); throw new Error("expected execute to reject"); } catch (error) { assert(error instanceof ConnectorActionsClientError); assertEqual(error.name, "ConnectorsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested connector action failure" } }); assertEqual(error.message, "nested connector action failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
