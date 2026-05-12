import { createConversationsControlClient, ConversationsControlClientError } from "./conversations-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConversationsControlClient deletes visible messages with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationsControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "deleted" }); } });
  const deleted = await client.deleteMessages("s1");
  assertEqual(deleted.status, "deleted"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/messages?session_id=s1"); assertEqual(calls[0]?.init?.method, "DELETE"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConversationsControlClient manages rename pin and archive operations with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationsControlClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "updated", session: { id: "s1", name: "新的会话", pinned: true } }); } });
  const renamed = await client.rename("s1", "新的会话"); await client.pin("s1"); await client.archive("s1", false);
  assertEqual(renamed.session?.name, "新的会话"); assertEqual(calls[0]?.init?.method, "PUT"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/manage"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { session_id: "s1", name: "新的会话" }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { session_id: "s1", pinned: true }); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { session_id: "s1", archive: false }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ConversationsControlClient exposes text control errors", async () => {
  const client = createConversationsControlClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET or DELETE only", { status: 405 }) });
  try { await client.deleteMessages("s1"); throw new Error("expected deleteMessages to reject"); } catch (error) { assert(error instanceof ConversationsControlClientError); assertEqual(error.name, "ConversationsClientError"); assertEqual(error.status, 405); assertEqual(error.body, "GET or DELETE only"); assertEqual(error.message, "GET or DELETE only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
