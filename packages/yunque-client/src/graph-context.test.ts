import { createGraphContextClient, GraphContextClientError } from "./graph-context";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("GraphContextClient reads context by entity id with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphContextClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ context: "云雀 -> Agent", neighbors: [{ id: "e2" }] }); } });
  const result = await client.byEntityId("e1");
  assertEqual(result.context, "云雀 -> Agent");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/context?entity_id=e1");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("GraphContextClient reads context by name with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphContextClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ context: "云雀", neighbors: [{ id: "e1" }] }); } });
  const result = await client.byName("云雀 Agent");
  assertEqual(result.neighbors?.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/context?name=%E4%BA%91%E9%9B%80+Agent");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("GraphContextClient exposes graph-context nested gateway errors", async () => {
  const client = createGraphContextClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "GRAPH_CONTEXT", message: "nested graph context failure" } }, { status: 404 }) });
  try { await client.byEntityId("missing"); throw new Error("expected context to reject"); } catch (error) { assert(error instanceof GraphContextClientError); assertEqual(error.name, "GraphClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { code: "GRAPH_CONTEXT", message: "nested graph context failure" } }); assertEqual(error.message, "nested graph context failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
