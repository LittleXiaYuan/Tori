import { createGraphReadClient, GraphReadClientError } from "./graph-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("GraphReadClient lists and searches entities with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createGraphReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entities: [{ id: "e1", name: "云雀", type: "project", properties: { lang: "go" }, mentions: 2 }] }); } });
  const result = await client.entities("云雀");
  assertEqual(result.entities[0]?.name, "云雀");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/entities?q=%E4%BA%91%E9%9B%80");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("GraphReadClient reads relations context and stats with API key", async () => {
  const calls: string[] = [];
  const client = createGraphReadClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url) => { calls.push(String(url)); if (String(url).includes("relations")) return jsonResponse({ relations: [{ id: "r1", from_id: "e1", to_id: "e2", type: "uses" }] }); if (String(url).includes("stats")) return jsonResponse({ entities: 2, relations: 1 }); return jsonResponse({ context: "云雀 -> Agent", neighbors: [{ id: "e2" }] }); } });
  assertEqual((await client.relations("e1")).relations[0]?.id, "r1");
  assertEqual((await client.contextByEntityId("e1")).context, "云雀 -> Agent");
  assertEqual((await client.contextByName("云雀")).neighbors?.length, 1);
  assertEqual((await client.stats()).relations, 1);
  assertEqual(calls[0], "http://localhost:9090/v1/graph/relations?entity_id=e1");
  assertEqual(calls[1], "http://localhost:9090/v1/graph/context?entity_id=e1");
  assertEqual(calls[2], "http://localhost:9090/v1/graph/context?name=%E4%BA%91%E9%9B%80");
});

test("GraphReadClient exposes nested read errors", async () => {
  const client = createGraphReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "GRAPH_READ", message: "read failed" } }, { status: 500 }) });
  try { await client.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof GraphReadClientError); assertEqual(error.name, "GraphClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "GRAPH_READ", message: "read failed" } }); assertEqual(error.message, "read failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
