import { createMissionsParseClient, MissionsParseClientError } from "./missions-parse";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("MissionsParseClient parses natural-language missions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMissionsParseClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ type: "cron", name: "每日总结", description: "每天总结", config: { cron_expr: "0 8 * * *" }, confidence: 0.9, explanation: "mentions daily" }); } });
  const result = await client.parse("每天八点总结昨天的工作");
  assertEqual(result.type, "cron"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/missions/parse"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { description: "每天八点总结昨天的工作" });
});

test("MissionsParseClient exposes nested parse errors", async () => {
  const client = createMissionsParseClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "description is required" } }, { status: 400 }) });
  try { await client.parse(""); throw new Error("expected parse to reject"); } catch (error) { assert(error instanceof MissionsParseClientError); assertEqual(error.name, "MissionsClientError"); assertEqual(error.status, 400); assertEqual(error.message, "description is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
