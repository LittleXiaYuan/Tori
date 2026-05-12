import { createKnowledgeSourceReadClient, KnowledgeSourceReadClientError } from "./knowledge-source-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("KnowledgeSourceReadClient reads stats and sources with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeSourceReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/stats")) return jsonResponse({ sources: 1, chunks: 3 }); return jsonResponse({ sources: [{ id: "src-1", name: "roadmap.md" }] }); } });
  const stats = await client.stats(); const sources = await client.list();
  assertEqual(stats.chunks, 3); assertEqual(sources.sources[0]?.id, "src-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/stats"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/knowledge/sources"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("KnowledgeSourceReadClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeSourceReadClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ sources: [] }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("KnowledgeSourceReadClient exposes nested source read errors", async () => {
  const client = createKnowledgeSourceReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested knowledge source read failure" } }, { status: 404 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof KnowledgeSourceReadClientError); assertEqual(error.name, "KnowledgeClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "nested knowledge source read failure" } }); assertEqual(error.message, "nested knowledge source read failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
