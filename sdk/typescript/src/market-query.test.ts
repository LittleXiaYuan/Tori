import { createSkillMarketQueryClient, SkillMarketQueryClientError } from "./market-query";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillMarketQueryClient searches skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketQueryClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "doc_parse", version: "1.0.0", category: "data", tags: ["docx"] }], count: 1 }); } });
  const result = await client.search("docx");
  assertEqual(result.skills[0]?.name, "doc_parse");
  assertEqual(result.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/search?q=docx");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillMarketQueryClient lists all skills when query is omitted with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketQueryClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "web_search", version: "1.0.0", installs: 12 }] }); } });
  const result = await client.search();
  assertEqual(result.skills[0]?.name, "web_search");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/search");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SkillMarketQueryClient exposes market-query nested gateway errors", async () => {
  const client = createSkillMarketQueryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested market query failure" } }, { status: 400 }) });
  try { await client.search(""); throw new Error("expected search to reject"); } catch (error) { assert(error instanceof SkillMarketQueryClientError); assertEqual(error.name, "SkillMarketClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested market query failure" } }); assertEqual(error.message, "nested market query failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
