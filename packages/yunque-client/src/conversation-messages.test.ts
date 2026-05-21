import { createConversationMessagesClient, ConversationMessagesClientError } from "./conversation-messages";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConversationMessagesClient lists messages with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationMessagesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ messages: [{ role: "user", content: "你好呀" }], count: 1 }); } });
  const result = await client.list("s1");
  assertEqual(result.messages[0]?.content, "你好呀"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/messages?session_id=s1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConversationMessagesClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationMessagesClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ messages: [], count: 0 }); } });
  await client.list("s1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ConversationMessagesClient exposes nested message errors", async () => {
  const client = createConversationMessagesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested conversation message failure" } }, { status: 400 }) });
  try { await client.list(""); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ConversationMessagesClientError); assertEqual(error.name, "ConversationsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { message: "nested conversation message failure" } }); assertEqual(error.message, "nested conversation message failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
