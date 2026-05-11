import { createSkillMarketClient, SkillMarketClientError } from "./market";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillMarketClient searches skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "doc_parse", version: "1.0.0", category: "data", tags: ["docx"] }], count: 1 }); } });
  const result = await client.search("docx");
  assertEqual(result.skills[0]?.name, "doc_parse"); assertEqual(result.count, 1); assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/search?q=docx"); assertEqual(calls[0]?.init?.method, "GET"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillMarketClient lists all skills when query is omitted", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "web_search", version: "1.0.0", installs: 12 }] }); } });
  const result = await client.search();
  assertEqual(result.skills[0]?.name, "web_search"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/search"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SkillMarketClient reads popular and rated top lists", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "code_gen", version: "2.1.0", rating: 4.8 }] }); } });
  await client.top({ n: 3 }); const rated = await client.top({ n: 5, by: "rating" });
  assertEqual(rated.skills[0]?.name, "code_gen"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/top?n=3"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/market/top?n=5&by=rating");
});

test("SkillMarketClient reads marketplace stats", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ total: 5, deprecated: 1, total_installs: 42, categories: { coding: 2 } }); } });
  const stats = await client.stats();
  assertEqual(stats.total, 5); assertDeepEqual(stats.categories, { coding: 2 }); assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/stats");
});

test("SkillMarketClient throws SkillMarketClientError with parsed and text bodies", async () => {
  const jsonClient = createSkillMarketClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "skill market not configured" }, { status: 503 }) });
  try { await jsonClient.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof SkillMarketClientError); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: "skill market not configured" }); assertEqual(error.message, "skill market not configured"); }
  const nestedClient = createSkillMarketClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "market query is required" } }, { status: 400 }) });
  try { await nestedClient.search(""); throw new Error("expected search to reject"); } catch (error) { assert(error instanceof SkillMarketClientError); assertEqual(error.status, 400); assertEqual(error.message, "market query is required"); }
  const textClient = createSkillMarketClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.search("x"); throw new Error("expected search to reject"); } catch (error) { assert(error instanceof SkillMarketClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
