import { createMemorySearchClient, MemorySearchClientError } from "./memory-search";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("MemorySearchClient searches memory with compact string API", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemorySearchClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ results: [{ key: "pref", value: "喜欢中文回复" }], count: 1 });
    },
  });

  const result = await client.search("中文回复", { limit: 3, layer: "all" });

  assertEqual(result.count, 1);
  assertEqual(result.results[0]?.key, "pref");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory/search");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ limit: 3, layer: "all", query: "中文回复" }));
});

test("MemorySearchClient accepts full MemorySearchRequest", async () => {
  const client = createMemorySearchClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      assertEqual(init?.body, JSON.stringify({ query: "planner", limit: 2, layer: "long" }));
      return jsonResponse({ results: [], count: 0 });
    },
  });

  const result = await client.search({ query: "planner", limit: 2, layer: "long" });

  assertEqual(result.count, 0);
});

test("MemorySearchClient exposes memory-search-named nested gateway errors", async () => {
  const client = createMemorySearchClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested memory search failure" } }, { status: 503 }),
  });

  try {
    await client.search("anything");
    throw new Error("expected search to reject");
  } catch (error) {
    assert(error instanceof MemorySearchClientError);
    assertEqual(error.status, 503);
    assertEqual(error.message, "nested memory search failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
