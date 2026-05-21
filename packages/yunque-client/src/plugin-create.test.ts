import { createPluginCreateClient, PluginCreateClientError } from "./plugin-create";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginCreateClient creates plugins with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginCreateClient({ baseUrl: "http://localhost:9090/", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "created", name: "demo", dir: "demo", full_path: "C:/tmp/demo" }, { status: 201 }); } });
  const result = await client.create({ name: "demo", language: "python", template: "basic", skills: [{ name: "hello" }] });
  assertEqual(result.status, "created");
  assertEqual(result.full_path, "C:/tmp/demo");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/create");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "demo", language: "python", template: "basic", skills: [{ name: "hello" }] });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("PluginCreateClient exposes plugin-create nested gateway errors", async () => {
  const client = createPluginCreateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "PLUGIN_CREATE", message: "nested plugin create failure" } }, { status: 400 }) });
  try { await client.create({ name: "" }); throw new Error("expected create to reject"); } catch (error) { assert(error instanceof PluginCreateClientError); assertEqual(error.name, "PluginsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "PLUGIN_CREATE", message: "nested plugin create failure" } }); assertEqual(error.message, "nested plugin create failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
