import { createEmotionHistoryClient, EmotionHistoryClientError } from "./emotion-history";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("EmotionHistoryClient reads filtered emotion history with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createEmotionHistoryClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entries: [{ emotion: "happy" }], summary: { happy: 1 }, total: 1 }); } });
  const history = await client.history({ sessionId: "s1", limit: 5, from: "2026-05-11T00:00:00Z" });
  assertEqual(history.total, 1); assertEqual(history.entries[0]?.emotion, "happy"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/emotion/history?session_id=s1&limit=5&from=2026-05-11T00%3A00%3A00Z"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("EmotionHistoryClient exposes nested history errors", async () => {
  const client = createEmotionHistoryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "session is invalid" } }, { status: 400 }) });
  try { await client.history({ sessionId: "bad" }); throw new Error("expected history to reject"); } catch (error) { assert(error instanceof EmotionHistoryClientError); assertEqual(error.name, "InteractionsClientError"); assertEqual(error.status, 400); assertEqual(error.message, "session is invalid"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
