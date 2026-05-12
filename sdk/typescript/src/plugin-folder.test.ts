import { createPluginFolderClient, PluginFolderClientError } from "./plugin-folder";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginFolderClient opens a named plugin folder with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginFolderClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, path: "C:/plugins/demo" }); } });
  assertEqual((await client.openFolder("demo")).ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/open-folder?name=demo");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PluginFolderClient opens root plugin folder without name", async () => {
  const calls: string[] = [];
  const client = createPluginFolderClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); return jsonResponse({ ok: true, path: "C:/plugins" }); } });
  await client.openFolder();
  assertEqual(calls[0], "http://localhost:9090/v1/plugins/open-folder");
});

test("PluginFolderClient exposes plugin-folder nested gateway errors", async () => {
  const client = createPluginFolderClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "PLUGIN_FOLDER", message: "nested plugin folder failure" } }, { status: 400 }) });
  try { await client.openFolder("missing"); throw new Error("expected openFolder to reject"); } catch (error) { assert(error instanceof PluginFolderClientError); assertEqual(error.name, "PluginsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "PLUGIN_FOLDER", message: "nested plugin folder failure" } }); assertEqual(error.message, "nested plugin folder failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
