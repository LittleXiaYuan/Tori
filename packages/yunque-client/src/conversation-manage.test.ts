import { createConversationManageClient, ConversationManageClientError } from "./conversation-manage";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConversationManageClient renames sessions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationManageClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "updated", session: { id: "s1", name: "新的会话" } }); } });
  const result = await client.rename("s1", "新的会话");
  assertEqual(result.session?.name, "新的会话"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations/manage"); assertEqual(calls[0]?.init?.method, "PUT"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { session_id: "s1", name: "新的会话" }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConversationManageClient pins archives and supports API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationManageClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "updated", session: { id: "s1", pinned: true } }); } });
  await client.pin("s1"); await client.archive("s1", false); await client.update({ session_id: "s1", name: "x" });
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { session_id: "s1", pinned: true }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { session_id: "s1", archive: false }); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { session_id: "s1", name: "x" }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ConversationManageClient exposes nested manage errors", async () => {
  const client = createConversationManageClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested conversation manage failure" } }, { status: 400 }) });
  try { await client.rename("", "x"); throw new Error("expected rename to reject"); } catch (error) { assert(error instanceof ConversationManageClientError); assertEqual(error.name, "ConversationsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { message: "nested conversation manage failure" } }); assertEqual(error.message, "nested conversation manage failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
