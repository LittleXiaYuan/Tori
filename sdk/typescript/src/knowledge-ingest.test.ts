import { createKnowledgeIngestClient, KnowledgeIngestClientError } from "./knowledge-ingest";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("KnowledgeIngestClient ingests explicit request with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeIngestClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ source: { id: "src-1", name: "roadmap.md" }, stats: { sources: 1, chunks: 3 } });
    },
  });

  const result = await client.ingest({ name: "roadmap.md", trigger: "manual", content: "SDK 增量包路线" });

  assertEqual(result.source?.id, "src-1");
  assertEqual(result.stats?.chunks, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/ingest");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ name: "roadmap.md", trigger: "manual", content: "SDK 增量包路线" }));
});

test("KnowledgeIngestClient ingestText offers compact write API with API key auth", async () => {
  const client = createKnowledgeIngestClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      assertEqual(init?.body, JSON.stringify({ name: "note.md", trigger: "chat", content: "Planner 需要先恢复上下文" }));
      return jsonResponse({ source: { id: "src-note" }, stats: { sources: 1 } });
    },
  });

  const result = await client.ingestText("Planner 需要先恢复上下文", { name: "note.md", trigger: "chat" });

  assertEqual(result.source?.id, "src-note");
});

test("KnowledgeIngestClient exposes knowledge-ingest-named nested gateway errors", async () => {
  const client = createKnowledgeIngestClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested knowledge ingest failure" } }, { status: 400 }),
  });

  try {
    await client.ingestText("");
    throw new Error("expected ingestText to reject");
  } catch (error) {
    assert(error instanceof KnowledgeIngestClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "nested knowledge ingest failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
