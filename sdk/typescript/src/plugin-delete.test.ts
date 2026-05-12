import { createPluginDeleteClient, PluginDeleteClientError } from "./plugin-delete";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginDeleteClient deletes plugins with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginDeleteClient({ baseUrl: "http://localhost:9090/", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "deleted", name: "demo" }); } });
  const result = await client.delete("demo plugin");
  assertEqual(result.status, "deleted");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/delete?name=demo+plugin");
  assertEqual(calls[0]?.init?.method, "DELETE");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("PluginDeleteClient exposes plugin-delete nested gateway errors", async () => {
  const client = createPluginDeleteClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "PLUGIN_DELETE", message: "nested plugin delete failure" } }, { status: 400 }) });
  try { await client.delete(""); throw new Error("expected delete to reject"); } catch (error) { assert(error instanceof PluginDeleteClientError); assertEqual(error.name, "PluginsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "PLUGIN_DELETE", message: "nested plugin delete failure" } }); assertEqual(error.message, "nested plugin delete failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
