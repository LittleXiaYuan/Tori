import { createConversationReplayClient, ConversationReplayClientError } from "./conversation-replay";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConversationReplayClient reads replay timeline with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationReplayClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ session_id: "s1", raw: true, turns: [{ turn: 1, assistant_reply: "hello" }], total_turns: 1 }); } });
  const result = await client.get("s1", { raw: true, limit: 10, offset: 2 });
  assertEqual(result.turns[0]?.assistant_reply, "hello"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/replay?session_id=s1&raw=true&limit=10&offset=2"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConversationReplayClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationReplayClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ session_id: "s1", turns: [], total_turns: 0 }); } });
  await client.get("s1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ConversationReplayClient exposes nested replay errors", async () => {
  const client = createConversationReplayClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested conversation replay failure" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof ConversationReplayClientError); assertEqual(error.name, "ConversationsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { message: "nested conversation replay failure" } }); assertEqual(error.message, "nested conversation replay failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
