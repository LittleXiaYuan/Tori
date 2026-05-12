import { createToriObserveClient, ToriObserveClientError } from "./tori-observe";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ToriObserveClient reads status health and usage with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToriObserveClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/status")) return jsonResponse({ bound: true, username: "alice" }); if (path.endsWith("/health")) return jsonResponse({ status: "ok" }); return jsonResponse({ total_tokens: 100 }); } });
  assertEqual((await client.status()).username, "alice");
  assertEqual((await client.health()).status, "ok");
  assertEqual((await client.usage()).total_tokens, 100);
  assertDeepEqual(calls.map((call) => call.url), ["http://localhost:9090/v1/tori/status", "http://localhost:9090/v1/tori/health", "http://localhost:9090/v1/tori/usage"]);
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ToriObserveClient supports API key and not-bound usage responses", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToriObserveClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ error: "not bound" }); } });
  assertEqual((await client.usage()).error, "not bound");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tori/usage");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ToriObserveClient exposes nested observe errors", async () => {
  const client = createToriObserveClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "TORI_OBSERVE", message: "observe failed" } }, { status: 500 }) });
  try { await client.status(); throw new Error("expected status to reject"); } catch (error) { assert(error instanceof ToriObserveClientError); assertEqual(error.name, "ToriClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "TORI_OBSERVE", message: "observe failed" } }); assertEqual(error.message, "observe failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
