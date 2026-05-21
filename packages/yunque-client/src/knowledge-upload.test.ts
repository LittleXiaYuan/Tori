import { createKnowledgeUploadClient, KnowledgeUploadClientError } from "./knowledge-upload";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("KnowledgeUploadClient uploads files as multipart with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createKnowledgeUploadClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ source: { id: "src-upload", name: "doc.md" }, stats: { sources: 1 } });
    },
  });

  const result = await client.uploadFile(new Blob(["# doc"]), "doc.md");

  assertEqual(result.source?.id, "src-upload");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/knowledge/upload");
  const headers = new Headers(calls[0]?.init?.headers);
  assertEqual(headers.get("authorization"), "Bearer token-123");
  assertEqual(headers.get("content-type"), null, "fetch must set multipart boundary automatically");
  assert(calls[0]?.init?.body instanceof FormData, "expected multipart FormData body");
});

test("KnowledgeUploadClient accepts full upload request and API key auth", async () => {
  const client = createKnowledgeUploadClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      assert(init?.body instanceof FormData, "expected FormData body");
      return jsonResponse({ source: { id: "src-upload-2" }, stats: { sources: 2 } });
    },
  });

  const result = await client.upload({ file: new Blob(["data"]), filename: "data.txt" });

  assertEqual(result.source?.id, "src-upload-2");
  assertEqual(result.stats?.sources, 2);
});

test("KnowledgeUploadClient exposes knowledge-upload-named nested gateway errors", async () => {
  const client = createKnowledgeUploadClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested knowledge upload failure" } }, { status: 413 }),
  });

  try {
    await client.uploadFile(new Blob(["too large"]), "large.txt");
    throw new Error("expected uploadFile to reject");
  } catch (error) {
    assert(error instanceof KnowledgeUploadClientError);
    assertEqual(error.status, 413);
    assertEqual(error.message, "nested knowledge upload failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
