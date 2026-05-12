import { createPluginKnowledgeIngestClient, PluginKnowledgeIngestClientError } from "./plugin-knowledge-ingest";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginKnowledgeIngestClient ingests plugin knowledge with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginKnowledgeIngestClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.ingest("content", "plugin", "note.md")).ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/knowledge/ingest");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { content: "content", source: "plugin", filename: "note.md" });
});

test("PluginKnowledgeIngestClient allows omitted source and filename", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginKnowledgeIngestClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true }); } });
  await client.ingest("content");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { content: "content" });
});

test("PluginKnowledgeIngestClient exposes plugin-knowledge-ingest nested gateway errors", async () => {
  const client = createPluginKnowledgeIngestClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_KNOWLEDGE_INGEST", message: "nested plugin knowledge ingest failure" } }, { status: 403 }) });
  try { await client.ingest("content"); throw new Error("expected ingest to reject"); } catch (error) { assert(error instanceof PluginKnowledgeIngestClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_KNOWLEDGE_INGEST", message: "nested plugin knowledge ingest failure" } }); assertEqual(error.message, "nested plugin knowledge ingest failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
