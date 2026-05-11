import { createConversationsClient, ConversationsClientError } from "./conversations";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConversationsClient lists sessions with archived filter and bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ sessions: [{ id: "s1", name: "Planner" }], count: 1 }); } });
  const result = await client.list({ archived: true });
  assertEqual(result.sessions[0]?.id, "s1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations?archived=true"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ConversationsClient reads and deletes visible messages with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ status: "deleted" }); return jsonResponse({ messages: [{ role: "user", content: "你好呀" }], count: 1 }); } });
  const messages = await client.messages("s1"); const deleted = await client.deleteMessages("s1");
  assertEqual(messages.messages[0]?.content, "你好呀"); assertEqual(deleted.status, "deleted"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/messages?session_id=s1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertEqual(calls[1]?.init?.method, "DELETE");
});

test("ConversationsClient manages rename pin and archive operations", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "updated", session: { id: "s1", name: "新的会话", pinned: true } }); } });
  const renamed = await client.rename("s1", "新的会话"); await client.pin("s1"); await client.archive("s1", false);
  assertEqual(renamed.session?.name, "新的会话"); assertEqual(calls[0]?.init?.method, "PUT"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { session_id: "s1", name: "新的会话" }); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { session_id: "s1", archive: false });
});

test("ConversationsClient reads replay timeline with pagination", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ session_id: "s1", raw: true, turns: [{ turn: 1, user_message: "hi", assistant_reply: "hello", pipeline: [] }], total_turns: 1 }); } });
  const replay = await client.replay("s1", { raw: true, limit: 10, offset: 2 });
  assertEqual(replay.turns[0]?.assistant_reply, "hello"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/replay?session_id=s1&raw=true&limit=10&offset=2");
});

test("ConversationsClient throws ConversationsClientError with text and JSON bodies", async () => {
  const jsonClient = createConversationsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "session_id is required" }, { status: 400 }) });
  try { await jsonClient.replay(""); throw new Error("expected replay to reject"); } catch (error) { assert(error instanceof ConversationsClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "session_id is required" }); assertEqual(error.message, "session_id is required"); }
  const textClient = createConversationsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET or DELETE only", { status: 405 }) });
  try { await textClient.messages("s1"); throw new Error("expected messages to reject"); } catch (error) { assert(error instanceof ConversationsClientError); assertEqual(error.status, 405); assertEqual(error.body, "GET or DELETE only"); assertEqual(error.message, "GET or DELETE only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
