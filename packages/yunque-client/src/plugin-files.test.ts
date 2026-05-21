import { createPluginFilesClient, PluginFilesClientError } from "./plugin-files";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginFilesClient reads plugin files with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginFilesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ files: [{ name: "plugin.json", content: "{}", size: 2 }], builtin: false }); } });
  const result = await client.files("demo");
  assertEqual(result.files[0]?.name, "plugin.json");
  assertEqual(result.files[0]?.size, 2);
  assertEqual(result.builtin, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/files?name=demo");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PluginFilesClient saves plugin files with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginFilesClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "saved" }); } });
  const result = await client.saveFile("demo", "plugin.json", "{\"name\":\"demo\"}");
  assertEqual(result.status, "saved");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/files?name=demo");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { file: "plugin.json", content: "{\"name\":\"demo\"}" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("PluginFilesClient preserves optional plugin field", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginFilesClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "saved" }); } });
  await client.saveFile("demo", "skill.py", "print('hi')", "python");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { plugin: "python", file: "skill.py", content: "print('hi')" });
});

test("PluginFilesClient exposes files nested gateway errors", async () => {
  const client = createPluginFilesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested plugin files failure" } }, { status: 400 }) });
  try { await client.files(""); throw new Error("expected files to reject"); } catch (error) { assert(error instanceof PluginFilesClientError); assertEqual(error.name, "PluginsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested plugin files failure" } }); assertEqual(error.message, "nested plugin files failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
