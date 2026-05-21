import { createBrowserOPPClient, BrowserOPPClientError } from "./browser-opp";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BrowserOPPClient lists pending OPP items with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserOPPClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ items: [{ problem_id: "opp-1", title: "Need approval" }], total: 1 }); } });
  const pending = await client.pending();
  assertEqual(pending.total, 1);
  assertEqual(pending.items[0]?.problem_id, "opp-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/browser/opp/pending");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("BrowserOPPClient posts OPP decisions with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserOPPClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "resolved", problem_id: "opp-1" }); } });
  const decided = await client.decide({ problem_id: "opp-1", decision: "allow_once" });
  assertEqual(decided.status, "resolved");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/browser/opp/decide");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { problem_id: "opp-1", decision: "allow_once" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("BrowserOPPClient exposes nested OPP errors", async () => {
  const client = createBrowserOPPClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested browser opp failure" } }, { status: 400 }) });
  try { await client.pending(); throw new Error("expected pending to reject"); } catch (error) { assert(error instanceof BrowserOPPClientError); assertEqual(error.name, "BrowserClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested browser opp failure" } }); assertEqual(error.message, "nested browser opp failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
