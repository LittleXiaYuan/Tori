import { createEmbeddingsClient, EmbeddingsClientError } from "./embeddings";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("EmbeddingsClient lists providers through embeddings facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createEmbeddingsClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ providers: ["local"] }); } });

  const result = await client.providers();

  assertEqual(result.providers[0], "local");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/embeddings");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("EmbeddingsClient embeds text without generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createEmbeddingsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ embedding: [0.1, 0.2], dimensions: 2, model: "local" }); } });

  const result = await client.embed("hello", "local");

  assertEqual(result.dimensions, 2);
  assertEqual(result.embedding[1], 0.2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/embeddings");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ text: "hello", provider: "local" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("EmbeddingsClient exposes embeddings-named errors", async () => {
  const client = createEmbeddingsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "embedding unavailable" } }, { status: 503 }) });

  try {
    await client.providers();
    throw new Error("expected providers to reject");
  } catch (error) {
    assert(error instanceof EmbeddingsClientError);
    assertEqual(error.status, 503);
    assertEqual(error.message, "embedding unavailable");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
