import { createConversationSessionsClient, ConversationSessionsClientError } from "./conversation-sessions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ConversationSessionsClient lists sessions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationSessionsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ sessions: [{ id: "s1", name: "Planner" }], count: 1 }); } });
  const result = await client.list({ archived: true });
  assertEqual(result.sessions[0]?.id, "s1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations?archived=true"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ConversationSessionsClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createConversationSessionsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ sessions: [], count: 0 }); } });
  await client.list();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/conversations"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ConversationSessionsClient exposes nested session errors", async () => {
  const client = createConversationSessionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested conversation session failure" } }, { status: 400 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ConversationSessionsClientError); assertEqual(error.name, "ConversationsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { message: "nested conversation session failure" } }); assertEqual(error.message, "nested conversation session failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
