import { createGraphEntitiesClient, GraphEntitiesClientError } from "./graph-entities";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("GraphEntitiesClient lists and searches entities with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphEntitiesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entities: [{ id: "e1", name: "云雀", type: "project", properties: { lang: "go" }, mentions: 2 }] }); } });
  const result = await client.entities("云雀");
  assertEqual(result.entities[0]?.name, "云雀");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/entities?q=%E4%BA%91%E9%9B%80");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("GraphEntitiesClient omits empty query and supports API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphEntitiesClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entities: [] }); } });
  assertDeepEqual(await client.entities(), { entities: [] });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/entities");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("GraphEntitiesClient exposes graph-entities nested gateway errors", async () => {
  const client = createGraphEntitiesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "GRAPH_ENTITIES", message: "nested graph entities failure" } }, { status: 500 }) });
  try { await client.entities("x"); throw new Error("expected entities to reject"); } catch (error) { assert(error instanceof GraphEntitiesClientError); assertEqual(error.name, "GraphClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "GRAPH_ENTITIES", message: "nested graph entities failure" } }); assertEqual(error.message, "nested graph entities failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
