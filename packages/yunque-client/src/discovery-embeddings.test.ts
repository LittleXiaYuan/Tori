import { createDiscoveryEmbeddingsClient, DiscoveryEmbeddingsClientError } from "./discovery-embeddings";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DiscoveryEmbeddingsClient lists embedding providers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDiscoveryEmbeddingsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ providers: ["mock"] }); } });
  const providers = await client.embeddingProviders();
  assertEqual(providers.providers[0], "mock"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/embeddings"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("DiscoveryEmbeddingsClient embeds text with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDiscoveryEmbeddingsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ embedding: [0.1, 0.2], dimensions: 2, model: "mock-embed" }); } });
  const embedded = await client.embed("云雀", "mock");
  assertEqual(embedded.dimensions, 2); assertEqual(calls[0]?.url, "http://localhost:9090/v1/embeddings"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { text: "云雀", provider: "mock" });
});

test("DiscoveryEmbeddingsClient exposes text and nested errors", async () => {
  const textClient = createDiscoveryEmbeddingsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("text is required", { status: 400 }) });
  try { await textClient.embed(""); throw new Error("expected embed to reject"); } catch (error) { assert(error instanceof DiscoveryEmbeddingsClientError); assertEqual(error.status, 400); assertEqual(error.body, "text is required"); assertEqual(error.message, "text is required"); }
  const nestedClient = createDiscoveryEmbeddingsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "embedding provider unavailable" } }, { status: 503 }) });
  try { await nestedClient.embeddingProviders(); throw new Error("expected embeddingProviders to reject"); } catch (error) { assert(error instanceof DiscoveryEmbeddingsClientError); assertEqual(error.name, "DiscoveryClientError"); assertEqual(error.status, 503); assertEqual(error.message, "embedding provider unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
