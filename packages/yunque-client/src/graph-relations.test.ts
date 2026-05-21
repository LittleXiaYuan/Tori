import { createGraphRelationsClient, GraphRelationsClientError } from "./graph-relations";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("GraphRelationsClient reads relations with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphRelationsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ relations: [{ id: "r1", from_id: "e1", to_id: "e2", type: "uses" }] }); } });
  const result = await client.relations("e1");
  assertEqual(result.relations[0]?.id, "r1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/relations?entity_id=e1");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("GraphRelationsClient encodes entity id and supports API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphRelationsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ relations: [] }); } });
  assertDeepEqual(await client.relations("e 1/二"), { relations: [] });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/relations?entity_id=e+1%2F%E4%BA%8C");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("GraphRelationsClient exposes graph-relations nested gateway errors", async () => {
  const client = createGraphRelationsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "GRAPH_RELATIONS", message: "nested graph relations failure" } }, { status: 500 }) });
  try { await client.relations("e1"); throw new Error("expected relations to reject"); } catch (error) { assert(error instanceof GraphRelationsClientError); assertEqual(error.name, "GraphClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "GRAPH_RELATIONS", message: "nested graph relations failure" } }); assertEqual(error.message, "nested graph relations failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
