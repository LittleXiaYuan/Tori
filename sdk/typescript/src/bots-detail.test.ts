import { createBotsDetailClient, BotsDetailClientError } from "./bots-detail";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BotsDetailClient gets bot detail with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsDetailClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ id: "bot-1", name: "reviewer", is_active: true }); } });
  const bot = await client.get("bot-1");
  assertEqual(bot.name, "reviewer"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/bots/detail?id=bot-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("BotsDetailClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsDetailClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ id: "bot-2", name: "planner" }); } });
  await client.get("bot-2");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("BotsDetailClient exposes nested detail errors", async () => {
  const client = createBotsDetailClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "NOT_FOUND", message: "nested bot not found" } }, { status: 404 }) });
  try { await client.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof BotsDetailClientError); assertEqual(error.name, "BotsClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { code: "NOT_FOUND", message: "nested bot not found" } }); assertEqual(error.message, "nested bot not found"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
