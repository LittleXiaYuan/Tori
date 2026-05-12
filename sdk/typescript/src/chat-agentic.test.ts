import { createChatAgenticClient, ChatAgenticClientError } from "./chat-agentic";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ChatAgenticClient sends agentic chat with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createChatAgenticClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ reply: "计划好了", plan: [{ id: 1, action: "inspect" }], steps: 1 }); } });
  const result = await client.agentic({ messages: [{ role: "user", content: "帮我规划" }], task_id: "t1" });
  assertEqual(result.reply, "计划好了");
  assertDeepEqual(result.plan, [{ id: 1, action: "inspect" }]);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chat/agentic");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ messages: [{ role: "user", content: "帮我规划" }], task_id: "t1" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ChatAgenticClient supports API key auth", async () => {
  const calls: { init?: RequestInit }[] = [];
  const client = createChatAgenticClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (_url, init) => { calls.push({ init }); return jsonResponse({ reply: "ok" }); } });
  assertEqual((await client.agentic({ messages: [{ role: "user", content: "run" }] })).reply, "ok");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ChatAgenticClient exposes nested agentic errors", async () => {
  const client = createChatAgenticClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "CHAT_AGENTIC", message: "agentic failed" } }, { status: 409 }) });
  try { await client.agentic({ messages: [] }); throw new Error("expected agentic to reject"); } catch (error) { assert(error instanceof ChatAgenticClientError); assertEqual(error.name, "ChatClientError"); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: { code: "CHAT_AGENTIC", message: "agentic failed" } }); assertEqual(error.message, "agentic failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
