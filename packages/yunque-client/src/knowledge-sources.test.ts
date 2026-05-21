import { createKnowledgeSourcesClient, KnowledgeSourcesClientError } from "./knowledge-sources";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("KnowledgeSourcesClient reads stats and sources with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeSourcesClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/stats")) return jsonResponse({ sources: 1, chunks: 3 });
      return jsonResponse({ sources: [{ id: "src-1", name: "roadmap.md" }] });
    },
  });

  const stats = await client.stats();
  const sources = await client.list();

  assertEqual(stats.chunks, 3);
  assertEqual(sources.sources[0]?.id, "src-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/stats");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/knowledge/sources");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("KnowledgeSourcesClient updates and deletes sources with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeSourcesClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      if (String(url).endsWith("/source/update")) return jsonResponse({ source: { id: "src-2", name: "updated.md" } });
      return jsonResponse({ deleted: "src-2", stats: { sources: 0 } });
    },
  });

  const updated = await client.update({ id: "src-2", name: "updated.md", content: "updated" });
  const deleted = await client.delete("src-2");

  assertEqual(updated.source?.name, "updated.md");
  assertEqual(deleted.deleted, "src-2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/source/update");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "src-2", name: "updated.md", content: "updated" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/knowledge/source?id=src-2");
  assertEqual(calls[1]?.init?.method, "DELETE");
});

test("KnowledgeSourcesClient exposes knowledge-sources-named nested gateway errors", async () => {
  const client = createKnowledgeSourcesClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested knowledge sources failure" } }, { status: 404 }),
  });

  try {
    await client.delete("missing");
    throw new Error("expected delete to reject");
  } catch (error) {
    assert(error instanceof KnowledgeSourcesClientError);
    assertEqual(error.status, 404);
    assertEqual(error.message, "nested knowledge sources failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
