import { createSkillMarketTopClient, SkillMarketTopClientError } from "./market-top";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillMarketTopClient reads popular top list with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketTopClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "code_gen", version: "2.1.0", installs: 99 }] }); } });
  const result = await client.top({ n: 3 });
  assertEqual(result.skills[0]?.name, "code_gen");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/top?n=3");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillMarketTopClient reads rated top list with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillMarketTopClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "doc_parse", version: "1.0.0", rating: 4.9 }] }); } });
  const result = await client.top({ n: 5, by: "rating" });
  assertEqual(result.skills[0]?.rating, 4.9);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/market/top?n=5&by=rating");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SkillMarketTopClient exposes market-top nested gateway errors", async () => {
  const client = createSkillMarketTopClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_GATEWAY", message: "nested market top failure" } }, { status: 502 }) });
  try { await client.top({ n: 1 }); throw new Error("expected top to reject"); } catch (error) { assert(error instanceof SkillMarketTopClientError); assertEqual(error.name, "SkillMarketClientError"); assertEqual(error.status, 502); assertDeepEqual(error.body, { error: { code: "BAD_GATEWAY", message: "nested market top failure" } }); assertEqual(error.message, "nested market top failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
