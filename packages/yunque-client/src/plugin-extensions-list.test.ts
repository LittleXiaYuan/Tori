import { createPluginExtensionsListClient, PluginExtensionsListClientError } from "./plugin-extensions-list";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginExtensionsListClient lists registered extensions with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginExtensionsListClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ extensions: [{ type: "provider" }] }); } });
  assertEqual((await client.list()).extensions.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/extensions");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
});

test("PluginExtensionsListClient exposes plugin-extensions-list nested gateway errors", async () => {
  const client = createPluginExtensionsListClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_EXTENSIONS_LIST", message: "nested plugin extensions list failure" } }, { status: 403 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof PluginExtensionsListClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_EXTENSIONS_LIST", message: "nested plugin extensions list failure" } }); assertEqual(error.message, "nested plugin extensions list failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
