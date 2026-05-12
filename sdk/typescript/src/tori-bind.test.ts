import { createToriBindClient, ToriBindClientError } from "./tori-bind";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ToriBindClient starts bind flow with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToriBindClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "pending", authorize_url: "https://tori.example/oauth" }); } });
  assertEqual((await client.bind({ tori_url: "https://tori.example" })).authorize_url, "https://tori.example/oauth");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tori/bind");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { tori_url: "https://tori.example" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ToriBindClient unbinds with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToriBindClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "not_bound" }); } });
  assertEqual((await client.unbind()).status, "not_bound");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tori/unbind");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ToriBindClient exposes nested bind errors", async () => {
  const client = createToriBindClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "TORI_BIND", message: "bind failed" } }, { status: 409 }) });
  try { await client.bind(); throw new Error("expected bind to reject"); } catch (error) { assert(error instanceof ToriBindClientError); assertEqual(error.name, "ToriClientError"); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: { code: "TORI_BIND", message: "bind failed" } }); assertEqual(error.message, "bind failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
