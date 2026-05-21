import { createKnowledgeSourceControlClient, KnowledgeSourceControlClientError } from "./knowledge-source-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("KnowledgeSourceControlClient updates sources with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeSourceControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ source: { id: "src-2", name: "updated.md" } }); } });
  const result = await client.update({ id: "src-2", name: "updated.md", content: "updated" });
  assertEqual(result.source?.name, "updated.md"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/source/update"); assertEqual(calls[0]?.init?.method, "POST"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "src-2", name: "updated.md", content: "updated" }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("KnowledgeSourceControlClient deletes sources with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeSourceControlClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ deleted: "src-2", stats: { sources: 0 } }); } });
  const result = await client.delete("src-2");
  assertEqual(result.deleted, "src-2"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/source?id=src-2"); assertEqual(calls[0]?.init?.method, "DELETE"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("KnowledgeSourceControlClient exposes nested source control errors", async () => {
  const client = createKnowledgeSourceControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested knowledge source control failure" } }, { status: 404 }) });
  try { await client.delete("missing"); throw new Error("expected delete to reject"); } catch (error) { assert(error instanceof KnowledgeSourceControlClientError); assertEqual(error.name, "KnowledgeClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "nested knowledge source control failure" } }); assertEqual(error.message, "nested knowledge source control failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
