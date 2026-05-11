import { createPluginCrudClient, PluginCrudClientError } from "./plugin-crud";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginCrudClient creates plugins with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginCrudClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "created", name: "demo", dir: "demo", full_path: "C:/plugins/demo" }, { status: 201 }); } });
  const result = await client.create({ name: "demo", language: "python", template: "basic", skills: [{ name: "hello" }] });
  assertEqual(result.status, "created");
  assertEqual(result.full_path, "C:/plugins/demo");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/create");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "demo", language: "python", template: "basic", skills: [{ name: "hello" }] });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PluginCrudClient deletes plugins with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginCrudClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "deleted", name: "demo" }); } });
  const result = await client.delete("demo");
  assertEqual(result.status, "deleted");
  assertEqual(result.name, "demo");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/delete?name=demo");
  assertEqual(calls[0]?.init?.method, "DELETE");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("PluginCrudClient preserves optional create fields", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginCrudClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "created", name: "demo", dir: "demo" }); } });
  await client.create({ name: "demo", description: "Demo", system_prompt: "Be helpful" });
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "demo", description: "Demo", system_prompt: "Be helpful" });
});

test("PluginCrudClient exposes crud nested gateway errors", async () => {
  const client = createPluginCrudClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested plugin crud failure" } }, { status: 400 }) });
  try { await client.create({ name: "" }); throw new Error("expected create to reject"); } catch (error) { assert(error instanceof PluginCrudClientError); assertEqual(error.name, "PluginsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested plugin crud failure" } }); assertEqual(error.message, "nested plugin crud failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
