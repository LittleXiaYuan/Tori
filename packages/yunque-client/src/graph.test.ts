import { createGraphClient, GraphClientError } from "./graph";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("GraphClient lists and searches entities with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entities: [{ id: "e1", name: "云雀", type: "project", properties: { lang: "go" }, mentions: 2 }] }); } });
  const result = await client.entities("云雀");
  assertEqual(result.entities[0]?.name, "云雀"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/entities?q=%E4%BA%91%E9%9B%80"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("GraphClient writes and deletes entities with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ ok: true }); return jsonResponse({ id: "e1", name: "云雀", type: "project", properties: { stack: "agent" } }); } });
  const entity = await client.putEntity({ name: "云雀", type: "project", properties: { stack: "agent" } }); const deleted = await client.deleteEntity("e1");
  assertEqual(entity.id, "e1"); assertEqual(deleted.ok, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/entities"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/graph/entities?id=e1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "云雀", type: "project", properties: { stack: "agent" } });
});

test("GraphClient reads and writes relations", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "r1", from_id: "e1", to_id: "e2", type: "uses", weight: 0.9 }); return jsonResponse({ relations: [{ id: "r1", from_id: "e1", to_id: "e2", type: "uses" }] }); } });
  const relations = await client.relations("e1"); const relation = await client.putRelation({ from_id: "e1", to_id: "e2", type: "uses", weight: 0.9 });
  assertEqual(relations.relations[0]?.id, "r1"); assertEqual(relation.weight, 0.9); assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/relations?entity_id=e1"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { from_id: "e1", to_id: "e2", type: "uses", weight: 0.9 });
});

test("GraphClient reads context and stats", async () => {
  const calls: string[] = [];
  const client = createGraphClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); if (String(url).includes("stats")) return jsonResponse({ entities: 2, relations: 1 }); return jsonResponse({ context: "云雀 -> Agent", neighbors: [{ id: "e2" }] }); } });
  const byId = await client.contextByEntityId("e1"); const byName = await client.contextByName("云雀"); const stats = await client.stats();
  assertEqual(byId.context, "云雀 -> Agent"); assertEqual(byName.neighbors?.length, 1); assertEqual(stats.relations, 1); assertEqual(calls[0], "http://localhost:9090/v1/graph/context?entity_id=e1"); assertEqual(calls[1], "http://localhost:9090/v1/graph/context?name=%E4%BA%91%E9%9B%80");
});

test("GraphClient throws GraphClientError with parsed and text bodies", async () => {
  const jsonClient = createGraphClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "id required" }, { status: 400 }) });
  try { await jsonClient.deleteEntity(""); throw new Error("expected delete to reject"); } catch (error) { assert(error instanceof GraphClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "id required" }); assertEqual(error.message, "id required"); }
  const nestedClient = createGraphClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested id required" } }, { status: 400 }) });
  try { await nestedClient.deleteEntity(""); throw new Error("expected nested delete to reject"); } catch (error) { assert(error instanceof GraphClientError); assertEqual(error.status, 400); assertEqual(error.message, "nested id required"); }
  const textClient = createGraphClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.putRelation({ from_id: "e1", to_id: "e2", type: "uses" }); throw new Error("expected relation to reject"); } catch (error) { assert(error instanceof GraphClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
