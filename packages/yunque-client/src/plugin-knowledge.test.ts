import { createPluginKnowledgeClient, PluginKnowledgeClientError } from "./plugin-knowledge";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginKnowledgeClient searches and ingests knowledge with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginKnowledgeClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/search")) return jsonResponse({ results: [{ id: "k1" }] }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.search("sdk", 4)).results.length, 1);
  assertEqual((await client.ingest("content", "plugin", "note.md")).ok, true);
  await client.search("sdk"); await client.ingest("content");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/knowledge/search");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { query: "sdk", limit: 4 });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { content: "content", source: "plugin", filename: "note.md" });
  assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { query: "sdk" });
  assertDeepEqual(JSON.parse(String(calls[3]?.init?.body)), { content: "content" });
});

test("PluginKnowledgeClient exposes nested knowledge errors", async () => {
  const client = createPluginKnowledgeClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_KNOWLEDGE", message: "plugin knowledge scope is required" } }, { status: 403 }) });
  try { await client.search("sdk"); throw new Error("expected search to reject"); } catch (error) { assert(error instanceof PluginKnowledgeClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_KNOWLEDGE", message: "plugin knowledge scope is required" } }); assertEqual(error.message, "plugin knowledge scope is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
