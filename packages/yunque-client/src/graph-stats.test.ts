import { createGraphStatsClient, GraphStatsClientError } from "./graph-stats";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("GraphStatsClient reads stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphStatsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entities: 2, relations: 1 }); } });
  const result = await client.stats();
  assertEqual(result.entities, 2);
  assertEqual(result.relations, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/stats");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("GraphStatsClient reads stats with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphStatsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entities: 0, relations: 0 }); } });
  assertDeepEqual(await client.stats(), { entities: 0, relations: 0 });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/stats");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("GraphStatsClient exposes graph-stats nested gateway errors", async () => {
  const client = createGraphStatsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "GRAPH_STATS", message: "nested graph stats failure" } }, { status: 500 }) });
  try { await client.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof GraphStatsClientError); assertEqual(error.name, "GraphClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "GRAPH_STATS", message: "nested graph stats failure" } }); assertEqual(error.message, "nested graph stats failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
