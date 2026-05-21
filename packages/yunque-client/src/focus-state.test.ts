import { createFocusStateClient, FocusStateClientError } from "./focus-state";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FocusStateClient reads focus with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFocusStateClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ focus: "planner", topics: ["sdk"] }); } });
  const result = await client.get();
  assertEqual(result.focus, "planner");
  assertDeepEqual(result.topics, ["sdk"]);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/state/focus");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("FocusStateClient updates focus and topics with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFocusStateClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  const result = await client.update("sdk", ["graph", "plugin-api"]);
  assertEqual(result.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/state/focus");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { focus: "sdk", topics: ["graph", "plugin-api"] });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("FocusStateClient exposes focus-state nested gateway errors", async () => {
  const client = createFocusStateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested focus failure" } }, { status: 400 }) });
  try { await client.update(""); throw new Error("expected update to reject"); } catch (error) { assert(error instanceof FocusStateClientError); assertEqual(error.name, "StateClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested focus failure" } }); assertEqual(error.message, "nested focus failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
