import { createDiscoveryClient, DiscoveryClientError } from "./discovery";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DiscoveryClient resolves and lists identity profiles with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDiscoveryClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ unified_id: "u1", display_name: "小羽", channels: { feishu: "42" } }); return jsonResponse({ profiles: [{ unified_id: "u1" }] }); } });
  const profile = await client.resolveIdentity({ channel: "feishu", user_id: "42", display_name: "小羽" }); const profiles = await client.identityProfiles();
  assertEqual(profile.unified_id, "u1"); assertEqual(profiles.profiles[0]?.unified_id, "u1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { channel: "feishu", user_id: "42", display_name: "小羽" });
});

test("DiscoveryClient lists embedding providers and embeds text with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDiscoveryClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ embedding: [0.1, 0.2], dimensions: 2, model: "mock-embed" }); return jsonResponse({ providers: ["mock"] }); } });
  const providers = await client.embeddingProviders(); const embedded = await client.embed("云雀", "mock");
  assertEqual(providers.providers[0], "mock"); assertEqual(embedded.dimensions, 2); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { text: "云雀", provider: "mock" });
});

test("DiscoveryClient searches and lists search providers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDiscoveryClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("providers")) return jsonResponse({ enabled: true, providers: ["bing"] }); return jsonResponse({ results: [{ title: "云雀", url: "https://example.test", snippet: "agent" }] }); } });
  const results = await client.search("planner", { limit: 3, provider: "bing" }); const providers = await client.searchProviders();
  assertEqual((results.results as Array<{ title?: string }>)[0]?.title, "云雀"); assertEqual(providers.enabled, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/search?q=planner&limit=3&provider=bing");
});

test("DiscoveryClient throws DiscoveryClientError with parsed and text bodies", async () => {
  const jsonClient = createDiscoveryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "q is required" }, { status: 400 }) });
  try { await jsonClient.search(""); throw new Error("expected search to reject"); } catch (error) { assert(error instanceof DiscoveryClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "q is required" }); assertEqual(error.message, "q is required"); }
  const nestedClient = createDiscoveryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "identity channel is required" } }, { status: 400 }) });
  try { await nestedClient.resolveIdentity({ channel: "", user_id: "42" }); throw new Error("expected resolveIdentity to reject"); } catch (error) { assert(error instanceof DiscoveryClientError); assertEqual(error.status, 400); assertEqual(error.message, "identity channel is required"); }
  const textClient = createDiscoveryClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST required", { status: 405 }) });
  try { await textClient.resolveIdentity({ channel: "feishu", user_id: "42" }); throw new Error("expected resolveIdentity to reject"); } catch (error) { assert(error instanceof DiscoveryClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST required"); assertEqual(error.message, "POST required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
