import { createKnowledgeImportClient, KnowledgeImportClientError } from "./knowledge-import";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("KnowledgeImportClient imports URL sources with compact API and bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeImportClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ imported: 2, source: { id: "src-url" }, sources: [{ id: "src-url" }, { id: "src-child" }] });
    },
  });

  const result = await client.importUrlString("https://deepwiki.com/org/repo", { crawl_children: true, max_pages: 2 });

  assertEqual(result.imported, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/import-url");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ crawl_children: true, max_pages: 2, url: "https://deepwiki.com/org/repo" }));
});

test("KnowledgeImportClient imports repo sources with request object and API key", async () => {
  const client = createKnowledgeImportClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      assertEqual(String(url), "http://localhost:9090/v1/knowledge/import-repo");
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      assertEqual(init?.body, JSON.stringify({ path: "C:/Code/AI/云雀/yunque-agent", max_files: 10 }));
      return jsonResponse({ source: { id: "src-repo", type: "repo" }, stats: { files: 10 } });
    },
  });

  const result = await client.importRepo({ path: "C:/Code/AI/云雀/yunque-agent", max_files: 10 });

  assertEqual(result.source?.id, "src-repo");
  assertEqual(result.stats?.files, 10);
});

test("KnowledgeImportClient exposes knowledge-import-named nested gateway errors", async () => {
  const client = createKnowledgeImportClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested knowledge import failure" } }, { status: 502 }),
  });

  try {
    await client.importRepoPath("missing");
    throw new Error("expected importRepoPath to reject");
  } catch (error) {
    assert(error instanceof KnowledgeImportClientError);
    assertEqual(error.status, 502);
    assertEqual(error.message, "nested knowledge import failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
