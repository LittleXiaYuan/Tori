import { createPluginMemoryWriteClient, PluginMemoryWriteClientError } from "./plugin-memory-write";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginMemoryWriteClient sets plugin KV memory with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginMemoryWriteClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.set("foo", "bar")).ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/memory/set");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { key: "foo", value: "bar" });
});

test("PluginMemoryWriteClient deletes plugin KV memory", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginMemoryWriteClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.delete("foo")).ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/memory/delete");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { key: "foo" });
});

test("PluginMemoryWriteClient exposes plugin-memory-write nested gateway errors", async () => {
  const client = createPluginMemoryWriteClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_MEMORY_WRITE", message: "nested plugin memory write failure" } }, { status: 403 }) });
  try { await client.set("foo", "bar"); throw new Error("expected set to reject"); } catch (error) { assert(error instanceof PluginMemoryWriteClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_MEMORY_WRITE", message: "nested plugin memory write failure" } }); assertEqual(error.message, "nested plugin memory write failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
