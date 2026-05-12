import { createConversationMessageControlClient, ConversationMessageControlClientError } from "./conversation-message-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConversationMessageControlClient deletes visible messages with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationMessageControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "deleted" }); } });
  const result = await client.delete("s1");
  assertEqual(result.status, "deleted"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/messages?session_id=s1"); assertEqual(calls[0]?.init?.method, "DELETE"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConversationMessageControlClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationMessageControlClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "deleted" }); } });
  await client.delete("s1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ConversationMessageControlClient exposes text control errors", async () => {
  const client = createConversationMessageControlClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET or DELETE only", { status: 405 }) });
  try { await client.delete("s1"); throw new Error("expected delete to reject"); } catch (error) { assert(error instanceof ConversationMessageControlClientError); assertEqual(error.name, "ConversationsClientError"); assertEqual(error.status, 405); assertEqual(error.body, "GET or DELETE only"); assertEqual(error.message, "GET or DELETE only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
