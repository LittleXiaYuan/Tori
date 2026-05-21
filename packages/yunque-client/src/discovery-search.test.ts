import { createDiscoverySearchClient, DiscoverySearchClientError } from "./discovery-search";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DiscoverySearchClient searches with q limit and provider query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDiscoverySearchClient({ baseUrl: "http://localhost:9090/", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ results: [{ title: "云雀", url: "https://example.test", snippet: "planner" }], total: 1 }); } });
  const results = await client.search("planner", { limit: 3, provider: "bing" });
  assertEqual((results.results as Array<{ title?: string }>)[0]?.title, "云雀"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/search?q=planner&limit=3&provider=bing");
});

test("DiscoverySearchClient lists search providers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDiscoverySearchClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ enabled: true, providers: ["bing"] }); } });
  const providers = await client.searchProviders();
  assertEqual(providers.enabled, true); assertEqual(providers.providers[0], "bing"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/search/providers"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("DiscoverySearchClient exposes nested and text search errors", async () => {
  const nestedClient = createDiscoverySearchClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "q is required" } }, { status: 400 }) });
  try { await nestedClient.search(""); throw new Error("expected search to reject"); } catch (error) { assert(error instanceof DiscoverySearchClientError); assertEqual(error.name, "DiscoveryClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "q is required" } }); assertEqual(error.message, "q is required"); }
  const textClient = createDiscoverySearchClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("search unavailable", { status: 503 }) });
  try { await textClient.searchProviders(); throw new Error("expected searchProviders to reject"); } catch (error) { assert(error instanceof DiscoverySearchClientError); assertEqual(error.status, 503); assertEqual(error.body, "search unavailable"); assertEqual(error.message, "search unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
