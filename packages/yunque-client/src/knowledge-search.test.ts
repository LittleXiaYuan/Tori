import { createKnowledgeSearchClient, KnowledgeSearchClientError } from "./knowledge-search";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("KnowledgeSearchClient searches knowledge with compact string API", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeSearchClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ chunks: [{ id: "chunk-1", content: "Planner 蓝图" }], count: 1 });
    },
  });

  const result = await client.search("Planner", { limit: 5, file: "blueprint.md", lang: "md" });

  assertEqual(result.count, 1);
  assertEqual(result.chunks[0]?.id, "chunk-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/search?q=Planner&n=5&file=blueprint.md&lang=md");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("KnowledgeSearchClient accepts full KnowledgeSearchRequest", async () => {
  const client = createKnowledgeSearchClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      assertEqual(String(url), "http://localhost:9090/v1/knowledge/search?q=RAG&n=2");
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      return jsonResponse({ chunks: [], count: 0 });
    },
  });

  const result = await client.search({ query: "RAG", limit: 2 });

  assertEqual(result.count, 0);
});

test("KnowledgeSearchClient exposes knowledge-search-named nested gateway errors", async () => {
  const client = createKnowledgeSearchClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested knowledge search failure" } }, { status: 400 }),
  });

  try {
    await client.search("");
    throw new Error("expected search to reject");
  } catch (error) {
    assert(error instanceof KnowledgeSearchClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "nested knowledge search failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
