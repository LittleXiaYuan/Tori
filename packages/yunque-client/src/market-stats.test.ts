import { createSkillMarketStatsClient, SkillMarketStatsClientError } from "./market-stats";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillMarketStatsClient reads marketplace stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketStatsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ total: 5, deprecated: 1, total_installs: 42, categories: { coding: 2 } }); } });
  const stats = await client.stats();
  assertEqual(stats.total, 5); assertDeepEqual(stats.categories, { coding: 2 }); assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/stats"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillMarketStatsClient reads marketplace stats with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketStatsClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ total: 2, categories: { data: 1, search: 1 } }); } });
  const stats = await client.stats();
  assertEqual(stats.total, 2); assertDeepEqual(stats.categories, { data: 1, search: 1 }); assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/stats"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SkillMarketStatsClient exposes nested stats errors through alias", async () => {
  const client = createSkillMarketStatsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "MARKET_STATS", message: "skill market not configured" } }, { status: 503 }) });
  try { await client.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof SkillMarketStatsClientError); assertEqual(error.name, "SkillMarketClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "MARKET_STATS", message: "skill market not configured" } }); assertEqual(error.message, "skill market not configured"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
