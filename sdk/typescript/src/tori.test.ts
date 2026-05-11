import { createToriClient, ToriClientError } from "./tori";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ToriClient starts bind flow with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToriClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "pending", authorize_url: "https://tori.example/oauth", message: "Please complete authorization in your browser" }); } });
  const result = await client.bind({ tori_url: "https://tori.example" });
  assertEqual(result.status, "pending"); assertEqual(result.authorize_url, "https://tori.example/oauth");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tori/bind"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(calls[0]?.init?.body, JSON.stringify({ tori_url: "https://tori.example" })); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ToriClient reads status health and usage with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToriClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/status")) return jsonResponse({ bound: true, username: "alice", tori_url: "https://tori.example", api_key_set: true }); if (path.endsWith("/health")) return jsonResponse({ status: "ok", version: "1.0" }); return jsonResponse({ total_tokens: 100, cost_cents: 12 }); } });
  const status = await client.status(); const health = await client.health(); const usage = await client.usage();
  assertEqual(status.bound, true); assertEqual(status.username, "alice"); assertEqual(health.status, "ok"); assertEqual((usage as { total_tokens?: number }).total_tokens, 100);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tori/status"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/tori/health"); assertEqual(calls[2]?.url, "http://localhost:9090/v1/tori/usage"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ToriClient unbinds and supports not-bound responses", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToriClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/unbind")) return jsonResponse({ status: "not_bound" }); return jsonResponse({ error: "not bound" }); } });
  const unbound = await client.unbind(); const usage = await client.usage();
  assertEqual(unbound.status, "not_bound"); assertEqual(usage.error, "not bound"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(calls[0]?.init?.body, JSON.stringify({}));
});

test("ToriClient throws ToriClientError with parsed and text bodies", async () => {
  const jsonClient = createToriClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "already bound, unbind first" }, { status: 409 }) });
  try { await jsonClient.bind(); throw new Error("expected bind to reject"); } catch (error) { assert(error instanceof ToriClientError); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: "already bound, unbind first" }); assertEqual(error.message, "already bound, unbind first"); }
  const textClient = createToriClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.unbind(); throw new Error("expected unbind to reject"); } catch (error) { assert(error instanceof ToriClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
