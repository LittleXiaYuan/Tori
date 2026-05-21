import { createPluginMemoryReadClient, PluginMemoryReadClientError } from "./plugin-memory-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginMemoryReadClient gets and lists plugin KV memory with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginMemoryReadClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/memory/get")) return jsonResponse({ value: "bar" }); return jsonResponse({ entries: [{ key: "foo" }] }); } });
  assertEqual((await client.get("foo")).value, "bar");
  assertEqual((await client.list("f")).entries.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/memory/get");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { key: "foo" });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { prefix: "f" });
});

test("PluginMemoryReadClient searches memory and allows omitted list prefix", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginMemoryReadClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/memory/search")) return jsonResponse({ results: [{ key: "foo" }] }); return jsonResponse({ entries: [] }); } });
  assertEqual((await client.search("foo", 2)).results.length, 1);
  await client.list();
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { query: "foo", limit: 2 });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), {});
});

test("PluginMemoryReadClient exposes plugin-memory-read nested gateway errors", async () => {
  const client = createPluginMemoryReadClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_MEMORY_READ", message: "nested plugin memory read failure" } }, { status: 403 }) });
  try { await client.get("foo"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof PluginMemoryReadClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_MEMORY_READ", message: "nested plugin memory read failure" } }); assertEqual(error.message, "nested plugin memory read failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
