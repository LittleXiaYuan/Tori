import { createRouterClient, RouterClientError } from "./router";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("RouterClient reads smart-router stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRouterClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ slots: { coding: { provider: "deepseek" } }, stats: { routed: 7, fallback: 1 } }); } });
  const result = await client.stats();
  assertDeepEqual(result.stats, { routed: 7, fallback: 1 }); assertDeepEqual(result.slots, { coding: { provider: "deepseek" } }); assertEqual(calls[0]?.url, "http://localhost:9090/v1/router/stats"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("RouterClient preserves not configured status with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRouterClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "not configured" }); } });
  const result = await client.stats();
  assertEqual(result.status, "not configured"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("RouterClient throws RouterClientError with parsed and text bodies", async () => {
  const jsonClient = createRouterClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "unauthorized" }, { status: 401 }) });
  try { await jsonClient.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof RouterClientError); assertEqual(error.status, 401); assertDeepEqual(error.body, { error: "unauthorized" }); assertEqual(error.message, "unauthorized"); }
  const nestedClient = createRouterClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "UNAUTHORIZED", message: "nested unauthorized" } }, { status: 401 }) });
  try { await nestedClient.stats(); throw new Error("expected nested stats to reject"); } catch (error) { assert(error instanceof RouterClientError); assertEqual(error.status, 401); assertEqual(error.message, "nested unauthorized"); }
  const textClient = createRouterClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof RouterClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
