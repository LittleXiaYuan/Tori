import { createGraphWriteClient, GraphWriteClientError } from "./graph-write";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("GraphWriteClient writes and deletes entities with bearer token", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createGraphWriteClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init, body: init?.body ? JSON.parse(String(init.body)) : undefined }); if (init?.method === "DELETE") return jsonResponse({ ok: true }); return jsonResponse({ id: "e1", name: "云雀", type: "project", properties: { stack: "agent" } }); } });
  const entity = await client.putEntity({ name: "云雀", type: "project", properties: { stack: "agent" } });
  const deleted = await client.deleteEntity("e1");
  assertEqual(entity.id, "e1");
  assertEqual(deleted.ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/entities");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/graph/entities?id=e1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(calls[0]?.body, { name: "云雀", type: "project", properties: { stack: "agent" } });
});

test("GraphWriteClient writes relations with API key", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createGraphWriteClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ id: "r1", from_id: "e1", to_id: "e2", type: "uses", weight: 0.9 }); } });
  const relation = await client.putRelation({ from_id: "e1", to_id: "e2", type: "uses", weight: 0.9 });
  assertEqual(relation.weight, 0.9);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/graph/relations");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(calls[0]?.body, { from_id: "e1", to_id: "e2", type: "uses", weight: 0.9 });
});

test("GraphWriteClient exposes nested write errors", async () => {
  const client = createGraphWriteClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "GRAPH_WRITE", message: "id required" } }, { status: 400 }) });
  try { await client.deleteEntity(""); throw new Error("expected delete to reject"); } catch (error) { assert(error instanceof GraphWriteClientError); assertEqual(error.name, "GraphClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "GRAPH_WRITE", message: "id required" } }); assertEqual(error.message, "id required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
