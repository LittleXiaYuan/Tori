import { createConversationsReadClient, ConversationsReadClientError } from "./conversations-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConversationsReadClient lists sessions and reads messages with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationsReadClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("messages")) return jsonResponse({ messages: [{ role: "user", content: "你好呀" }], count: 1 }); return jsonResponse({ sessions: [{ id: "s1", name: "Planner" }], count: 1 }); } });
  const list = await client.list({ archived: true }); const messages = await client.messages("s1");
  assertEqual(list.sessions[0]?.id, "s1"); assertEqual(messages.messages[0]?.content, "你好呀"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations?archived=true"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/conversations/messages?session_id=s1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ConversationsReadClient reads replay timeline with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationsReadClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ session_id: "s1", raw: true, turns: [{ turn: 1, user_message: "hi", assistant_reply: "hello", pipeline: [] }], total_turns: 1 }); } });
  const replay = await client.replay("s1", { raw: true, limit: 10, offset: 2 });
  assertEqual(replay.turns[0]?.assistant_reply, "hello"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/replay?session_id=s1&raw=true&limit=10&offset=2"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ConversationsReadClient exposes nested read errors", async () => {
  const client = createConversationsReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "conversation session id is required" } }, { status: 400 }) });
  try { await client.messages(""); throw new Error("expected messages to reject"); } catch (error) { assert(error instanceof ConversationsReadClientError); assertEqual(error.name, "ConversationsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "conversation session id is required" } }); assertEqual(error.message, "conversation session id is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
