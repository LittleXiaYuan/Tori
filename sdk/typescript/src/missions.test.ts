import { createMissionsClient, MissionsClientError } from "./missions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("MissionsClient parses natural-language missions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMissionsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ type: "cron", name: "每日总结", description: "每天总结", config: { cron_expr: "0 8 * * *", message: "总结昨天" }, confidence: 0.9, explanation: "mentions daily" }); } });
  const result = await client.parse("每天八点总结昨天的工作");
  assertEqual(result.type, "cron"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/missions/parse"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { description: "每天八点总结昨天的工作" });
});

test("MissionsClient reads experiences with search and filters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMissionsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ experiences: [{ id: "e1", source: "task", category: "sdk", outcome: "success" }], total: 1 }); } });
  const result = await client.experiences({ q: "sdk", source: "task", category: "sdk", outcome: "success", limit: 5 });
  assertEqual(result.total, 1); assertEqual(result.experiences[0]?.outcome, "success"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/reflect/experiences?q=sdk&source=task&category=sdk&outcome=success&limit=5"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("MissionsClient reads experience stats and strategies", async () => {
  const calls: string[] = [];
  const client = createMissionsClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); if (String(url).includes("stats=true")) return jsonResponse({ total: 10, by_outcome: { success: 8 }, recent_7d: 3 }); return jsonResponse({ strategies: "- 推荐: Prefer small slices" }); } });
  const stats = await client.experienceStats(); const strategies = await client.strategies();
  assertEqual(stats.total, 10); assertEqual(stats.by_outcome?.success, 8); assertEqual(strategies.strategies.includes("Prefer small slices"), true); assertEqual(calls[0], "http://localhost:9090/v1/reflect/experiences?stats=true"); assertEqual(calls[1], "http://localhost:9090/v1/reflect/strategies");
});

test("MissionsClient throws MissionsClientError with parsed and text bodies", async () => {
  const jsonClient = createMissionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "description is required" }, { status: 400 }) });
  try { await jsonClient.parse(""); throw new Error("expected parse to reject"); } catch (error) { assert(error instanceof MissionsClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "description is required" }); assertEqual(error.message, "description is required"); }
  const nestedClient = createMissionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested description is required" } }, { status: 400 }) });
  try { await nestedClient.parse(""); throw new Error("expected nested parse to reject"); } catch (error) { assert(error instanceof MissionsClientError); assertEqual(error.status, 400); assertEqual(error.message, "nested description is required"); }
  const textClient = createMissionsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("experience store not initialized", { status: 404 }) });
  try { await textClient.strategies(); throw new Error("expected strategies to reject"); } catch (error) { assert(error instanceof MissionsClientError); assertEqual(error.status, 404); assertEqual(error.body, "experience store not initialized"); assertEqual(error.message, "experience store not initialized"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
