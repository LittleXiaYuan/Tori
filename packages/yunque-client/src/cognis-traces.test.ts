import { createCognisTracesClient, CognisTracesClientError } from "./cognis-traces";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisTracesClient lists and reads traces with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisTracesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ traces: [{ id: "t1" }], count: 1 }); } });
  assertEqual((await client.list(10)).count, 1);
  assertEqual((await client.get("cogni/id", 5)).traces?.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/traces?limit=10");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/cogni%2Fid/trace?limit=5");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CognisTracesClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisTracesClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ events: [] }); } });
  await client.list();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/traces");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CognisTracesClient exposes nested trace errors", async () => {
  const client = createCognisTracesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "TRACE_MISSING", message: "trace scope is required" } }, { status: 403 }) });
  try { await client.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof CognisTracesClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "TRACE_MISSING", message: "trace scope is required" } }); assertEqual(error.message, "trace scope is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
