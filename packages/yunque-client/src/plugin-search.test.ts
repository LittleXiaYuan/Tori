import { createPluginSearchClient, PluginSearchClientError } from "./plugin-search";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginSearchClient searches with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginSearchClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ results: [{ title: "doc" }] }); } });
  const result = await client.search("agent", 3);
  assertEqual(result.results.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/search");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { query: "agent", limit: 3 });
});

test("PluginSearchClient allows omitted limit", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginSearchClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ results: [] }); } });
  await client.search("agent");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { query: "agent" });
});

test("PluginSearchClient exposes nested plugin search errors", async () => {
  const client = createPluginSearchClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_SEARCH", message: "plugin search scope is required" } }, { status: 403 }) });
  try { await client.search("agent"); throw new Error("expected search to reject"); } catch (error) { assert(error instanceof PluginSearchClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_SEARCH", message: "plugin search scope is required" } }); assertEqual(error.message, "plugin search scope is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
