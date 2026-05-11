import { createKnowledgeClient, KnowledgeClientError } from "./knowledge";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
}

const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];

function test(name: string, fn: () => Promise<void> | void): void {
  tests.push({ name, fn });
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

test("KnowledgeClient searches with backend query parameters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ chunks: [{ id: "chunk-1", content: "Planner 恢复" }], count: 1 });
    },
  });

  const result = await client.search({ query: "Planner", limit: 5, file: "blueprint.md", lang: "md" });

  assertEqual(result.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/search?q=Planner&n=5&file=blueprint.md&lang=md");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("KnowledgeClient ingests inline text with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ source: { id: "src-1", name: "roadmap.md" }, stats: { sources: 1, chunks: 3 } });
    },
  });

  const result = await client.ingest({ name: "roadmap.md", trigger: "manual", content: "SDK 增量包路线" });

  assertEqual(result.source?.id, "src-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/ingest");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ name: "roadmap.md", trigger: "manual", content: "SDK 增量包路线" }));
});

test("KnowledgeClient updates and deletes sources", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/source/update")) {
        return jsonResponse({ source: { id: "src-2", name: "updated.md" }, stats: { sources: 1 } });
      }
      return jsonResponse({ deleted: "src-2", stats: { sources: 0 } });
    },
  });

  const updated = await client.updateSource({ id: "src-2", name: "updated.md", content: "updated" });
  const deleted = await client.deleteSource("src-2");

  assertEqual(updated.source?.name, "updated.md");
  assertEqual(deleted.deleted, "src-2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/source/update");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "src-2", name: "updated.md", content: "updated" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/knowledge/source?id=src-2");
  assertEqual(calls[1]?.init?.method, "DELETE");
});

test("KnowledgeClient imports URL and repo sources", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/import-url")) {
        return jsonResponse({ imported: 2, source: { id: "src-url" }, sources: [{ id: "src-url" }, { id: "src-child" }] });
      }
      return jsonResponse({ source: { id: "src-repo", type: "repo" }, stats: { files: 10 } });
    },
  });

  const importedUrl = await client.importUrl({ url: "https://deepwiki.com/org/repo", crawl_children: true, max_pages: 2 });
  const importedRepo = await client.importRepo({ path: "C:/Code/AI/云雀/yunque-agent", max_files: 10 });

  assertEqual(importedUrl.imported, 2);
  assertEqual(importedRepo.source?.id, "src-repo");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ url: "https://deepwiki.com/org/repo", crawl_children: true, max_pages: 2 }));
  assertEqual(calls[1]?.init?.body, JSON.stringify({ path: "C:/Code/AI/云雀/yunque-agent", max_files: 10 }));
});

test("KnowledgeClient uploads files as multipart form data", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeClient({
    baseUrl: "http://localhost:9090",
    headers: { "X-Trace": "trace-1" },
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ source: { id: "src-upload", name: "doc.md" }, stats: { sources: 1 }, parse: { parser: "mineru" } });
    },
  });

  const result = await client.upload({ file: new Blob(["# doc"]), filename: "doc.md" });

  assertEqual(result.source?.id, "src-upload");
  assert(calls[0]?.init?.body instanceof FormData, "expected multipart FormData body");
  const headers = new Headers(calls[0]?.init?.headers);
  assertEqual(headers.get("x-trace"), "trace-1");
  assertEqual(headers.get("content-type"), null, "fetch must set multipart boundary automatically");
});

test("KnowledgeClient throws KnowledgeClientError with parsed body", async () => {
  const client = createKnowledgeClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "source not found" }, { status: 404 }),
  });

  try {
    await client.deleteSource("missing");
    throw new Error("expected deleteSource to reject");
  } catch (error) {
    assert(error instanceof KnowledgeClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "source not found" });
    assertEqual(error.message, "source not found");
  }
});

let failures = 0;
for (const { name, fn } of tests) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failures > 0) {
  process.exitCode = 1;
} else {
  console.log(`1..${tests.length}`);
}
