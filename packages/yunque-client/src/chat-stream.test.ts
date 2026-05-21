import { createChatStreamClient, ChatStreamClientError } from "./chat-stream";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
function streamFromText(text: string): ReadableStream<Uint8Array> { const encoder = new TextEncoder(); return new ReadableStream<Uint8Array>({ start(controller) { controller.enqueue(encoder.encode(text)); controller.close(); } }); }

test("ChatStreamClient streams SSE frames with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createChatStreamClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return new Response(streamFromText('data: {"content":"你"}\n\nevent: thinking\ndata: {"content":"思考"}\n\nevent: done\ndata: {"steps":1}\n\ndata: [DONE]\n\n'), { status: 200, headers: { "Content-Type": "text/event-stream" } }); } });
  const items = [];
  for await (const item of client.stream({ messages: [{ role: "user", content: "hi" }] })) items.push(item);
  assertDeepEqual(items, [
    { kind: "delta", content: "你" },
    { kind: "thinking", content: "思考", data: { content: "思考" }, raw: '{"content":"思考"}' },
    { kind: "done", data: { steps: 1 }, raw: '{"steps":1}' },
  ]);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chat/stream");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ messages: [{ role: "user", content: "hi" }], stream: true }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ChatStreamClient parses standalone streams and supports API key auth", async () => {
  const calls: { init?: RequestInit }[] = [];
  const client = createChatStreamClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (_url, init) => { calls.push({ init }); return new Response(streamFromText('event: actions\ndata: {"actions":[{"type":"open"}]}\n\n'), { status: 200 }); } });
  const parsed = [];
  for await (const item of client.parseStream(streamFromText('event: actions\ndata: {"actions":[{"type":"open"}]}\n\n'))) parsed.push(item);
  assertDeepEqual(parsed, [{ kind: "actions", actions: [{ type: "open" }], raw: '{"actions":[{"type":"open"}]}' }]);
  for await (const _ of client.stream({ messages: [{ role: "user", content: "run" }] })) { /* drain */ }
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ChatStreamClient exposes nested stream errors", async () => {
  const client = createChatStreamClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "CHAT_STREAM", message: "stream failed" } }, { status: 502 }) });
  try { for await (const _ of client.stream({ messages: [{ role: "user", content: "x" }] })) { /* noop */ } throw new Error("expected stream to reject"); } catch (error) { assert(error instanceof ChatStreamClientError); assertEqual(error.name, "ChatClientError"); assertEqual(error.status, 502); assertDeepEqual(error.body, { error: { code: "CHAT_STREAM", message: "stream failed" } }); assertEqual(error.message, "stream failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
