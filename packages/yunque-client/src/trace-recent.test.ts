import { createTraceRecentClient, TraceRecentClientError } from "./trace-recent";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TraceRecentClient reads recent events with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTraceRecentClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ count: 1, events: [{ id: "evt-1", trace_id: "tr-1" }] }); } });
  const result = await client.recent({ limit: 20 });
  assertEqual(result.count, 1); assertEqual(result.events[0]?.trace_id, "tr-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/recent?limit=20"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TraceRecentClient supports raw query and API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTraceRecentClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ count: 1, raw: true, events: [{ id: "evt-2" }] }); } });
  const result = await client.recent({ raw: true });
  assertEqual(result.raw, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/recent?raw=true"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TraceRecentClient exposes nested recent errors", async () => {
  const client = createTraceRecentClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested recent trace failure" } }, { status: 400 }) });
  try { await client.recent(); throw new Error("expected recent to reject"); } catch (error) { assert(error instanceof TraceRecentClientError); assertEqual(error.name, "TraceClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested recent trace failure" } }); assertEqual(error.message, "nested recent trace failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
