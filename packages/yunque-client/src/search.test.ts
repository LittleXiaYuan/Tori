import { createSearchClient, SearchClientError } from "./search";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SearchClient searches through search facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSearchClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ results: [{ title: "云雀", url: "https://example.test" }], total: 1 }); } });

  const result = await client.search("云雀 Agent", { limit: 3, provider: "local" });

  assertEqual(Array.isArray(result.results), true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/search?q=%E4%BA%91%E9%9B%80+Agent&limit=3&provider=local");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SearchClient lists search providers without generated SDK", async () => {
  const client = createSearchClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { assertEqual(String(url), "http://localhost:9090/v1/search/providers"); assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123"); return jsonResponse({ enabled: true, providers: ["local"] }); } });

  const result = await client.providers();

  assertEqual(result.enabled, true);
  assertEqual(result.providers[0], "local");
});

test("SearchClient exposes search-named errors", async () => {
  const client = createSearchClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "search unavailable" } }, { status: 503 }) });

  try {
    await client.providers();
    throw new Error("expected providers to reject");
  } catch (error) {
    assert(error instanceof SearchClientError);
    assertEqual(error.status, 503);
    assertEqual(error.message, "search unavailable");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
