import { createPluginMemoryClient, PluginMemoryClientError } from "./plugin-memory";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginMemoryClient manages plugin KV memory with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginMemoryClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); const value = String(url); if (value.includes("/memory/get")) return jsonResponse({ value: "bar" }); if (value.includes("/memory/list")) return jsonResponse({ entries: [{ key: "foo" }] }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.get("foo")).value, "bar");
  assertEqual((await client.set("foo", "bar")).ok, true);
  assertEqual((await client.delete("foo")).ok, true);
  assertEqual((await client.list("f")).entries.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/memory/get");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { key: "foo", value: "bar" });
  assertDeepEqual(JSON.parse(String(calls[3]?.init?.body)), { prefix: "f" });
});

test("PluginMemoryClient searches plugin memory and allows omitted list prefix", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginMemoryClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/memory/search")) return jsonResponse({ results: [{ key: "foo" }] }); return jsonResponse({ entries: [] }); } });
  assertEqual((await client.search("foo", 2)).results.length, 1);
  await client.list();
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { query: "foo", limit: 2 });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), {});
});

test("PluginMemoryClient exposes nested plugin memory errors", async () => {
  const client = createPluginMemoryClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_MEMORY", message: "plugin memory scope is required" } }, { status: 403 }) });
  try { await client.get("foo"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof PluginMemoryClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_MEMORY", message: "plugin memory scope is required" } }); assertEqual(error.message, "plugin memory scope is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
