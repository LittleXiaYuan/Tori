import { createTraceByIdClient, TraceByIdClientError } from "./trace-by-id";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TraceByIdClient reads events by trace id with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTraceByIdClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ trace_id: "tr/complex", count: 1, events: [{ id: "evt-1" }] }); } });
  const result = await client.get("tr/complex");
  assertEqual(result.trace_id, "tr/complex"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/tr%2Fcomplex"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TraceByIdClient supports raw mode and API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTraceByIdClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ trace_id: "tr-1", count: 1, raw: true, events: [{ id: "evt-2" }] }); } });
  const result = await client.get("tr-1", { raw: true });
  assertEqual(result.raw, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/tr-1?raw=true"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TraceByIdClient exposes nested by-id errors", async () => {
  const client = createTraceByIdClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested trace id failure" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof TraceByIdClientError); assertEqual(error.name, "TraceClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested trace id failure" } }); assertEqual(error.message, "nested trace id failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
