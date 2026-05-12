import { createBotsListClient, BotsListClientError } from "./bots-list";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BotsListClient lists bots with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsListClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ bots: [{ id: "bot-1", name: "default" }], total: 1, active: 1 }); } });
  const list = await client.list();
  assertEqual(list.total, 1); assertEqual(calls[0]?.url, "http://localhost:9090/v1/bots"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("BotsListClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsListClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ bots: [], total: 0, active: 0 }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("BotsListClient exposes nested list errors", async () => {
  const client = createBotsListClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_GATEWAY", message: "nested bot list failed" } }, { status: 502 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof BotsListClientError); assertEqual(error.name, "BotsClientError"); assertEqual(error.status, 502); assertDeepEqual(error.body, { error: { code: "BAD_GATEWAY", message: "nested bot list failed" } }); assertEqual(error.message, "nested bot list failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
