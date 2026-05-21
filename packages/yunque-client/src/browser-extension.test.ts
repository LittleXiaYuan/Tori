import { createBrowserExtensionClient, BrowserExtensionClientError } from "./browser-extension";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected?: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BrowserExtensionClient creates extension sessions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserExtensionClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, ws_url: "ws://localhost:9090/ws/browser", ticket: "t1" }); } });
  const session = await client.session();
  assertEqual(session.ticket, "t1");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/browser/ext/session");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("BrowserExtensionClient runs actions and scenarios with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserExtensionClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/ext/action")) return jsonResponse({ ok: true, title: "Example" }); if (String(url).endsWith("/ext/scenarios")) return jsonResponse({ scenarios: [{ id: "open-page" }] }); return jsonResponse({ ok: true, scenario: "open-page", results: [{ ok: true }] }); } });
  const action = await client.action({ type: "browser_navigate", url: "https://example.com" });
  const scenarios = await client.scenarios();
  const run = await client.runScenario("open-page");
  assertEqual(action.title, "Example");
  assertEqual(scenarios.scenarios[0]?.id, "open-page");
  assertEqual(run.scenario, "open-page");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/browser/ext/action");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/browser/ext/scenarios");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { type: "browser_navigate", url: "https://example.com" });
  assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { scenario_id: "open-page" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("BrowserExtensionClient exposes nested extension errors", async () => {
  const client = createBrowserExtensionClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested browser extension failure" } }, { status: 400 }) });
  try { await client.action({ type: "" }); throw new Error("expected action to reject"); } catch (error) { assert(error instanceof BrowserExtensionClientError); assertEqual(error.name, "BrowserClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested browser extension failure" } }); assertEqual(error.message, "nested browser extension failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
