import { createAiriClient, AiriClientError } from "./airi";
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
async function collect<T>(it: AsyncIterable<T>): Promise<T[]> { const out: T[] = []; for await (const item of it) out.push(item); return out; }
function streamFromText(text: string): ReadableStream<Uint8Array> { const enc = new TextEncoder(); return new ReadableStream({ start(c) { c.enqueue(enc.encode(text)); c.close(); } }); }
function json(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

async function testAiriStatusAndModels() {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAiriClient({ baseUrl: "http://localhost:9090/", apiKey: "airi-key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return String(url).endsWith("/models") ? json({ object: "list", data: [{ id: "yunque-airi" }] }) : json({ plugin: "airi", connected: false }); } });
  const status = await client.status(); const models = await client.models();
  assertEqual(status.connected, false); assertEqual(models.data[0]?.id, "yunque-airi");
  assertEqual(calls[0].url, "http://localhost:9090/v1/ext/airi/status"); assertEqual(new Headers(calls[0].init?.headers).get("authorization"), "Bearer airi-key");
  console.log("ok - AiriClient reads status and OpenAI-compatible models");
}

async function testAiriChatCompletions() {
  const calls: { body?: string }[] = [];
  const client = createAiriClient({ baseUrl: "http://localhost:9090", fetch: async (_url, init) => { calls.push({ body: String(init?.body) }); return json({ choices: [{ message: { role: "assistant", content: "<|ACT {}|> 好呀" }, finish_reason: "stop" }] }); } });
  const resp = await client.chatCompletions({ model: "yunque-airi", messages: [{ role: "user", content: "hi" }], stream: true });
  assertEqual(resp.choices?.[0]?.message?.content, "<|ACT {}|> 好呀"); assertDeepEqual(JSON.parse(calls[0].body || "{}"), { model: "yunque-airi", messages: [{ role: "user", content: "hi" }], stream: false });
  console.log("ok - AiriClient posts non-streaming chat completions");
}

async function testAiriStreamsCompletions() {
  const client = createAiriClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response(streamFromText('data: {"choices":[{"delta":{"content":"hi"}}]}\n\ndata: [DONE]\n\n'), { status: 200 }) });
  const items = await collect(client.streamChatCompletions({ messages: [{ role: "user", content: "hi" }] }));
  assertEqual(items[0].kind, "chunk"); if (items[0].kind === "chunk") assertEqual(items[0].chunk.choices?.[0]?.delta?.content, "hi"); assertEqual(items[1].kind, "done");
  console.log("ok - AiriClient parses streaming chat completion chunks");
}

async function testAiriErrors() {
  const client = createAiriClient({ baseUrl: "http://localhost:9090", fetch: async () => json({ error: "planner error" }, { status: 500 }) });
  try { await client.status(); throw new Error("expected status to reject"); } catch (error) { assert(error instanceof AiriClientError); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: "planner error" }); assertEqual(error.message, "planner error"); }
  const nestedClient = createAiriClient({ baseUrl: "http://localhost:9090", fetch: async () => json({ error: { code: "LLM_UNAVAILABLE", message: "model channel unavailable" } }, { status: 503 }) });
  try { await nestedClient.chatCompletions({ messages: [{ role: "user", content: "hi" }] }); throw new Error("expected nested chat to reject"); } catch (error) { assert(error instanceof AiriClientError); assertEqual(error.status, 503); assertEqual(error.message, "model channel unavailable"); }
  const textClient = createAiriClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST required", { status: 405 }) });
  try { await collect(textClient.streamChatCompletions({ messages: [] })); throw new Error("expected stream to reject"); } catch (error) { assert(error instanceof AiriClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST required"); assertEqual(error.message, "POST required"); }
  console.log("ok - AiriClient throws AiriClientError with parsed and text bodies");
}

await testAiriStatusAndModels();
await testAiriChatCompletions();
await testAiriStreamsCompletions();
await testAiriErrors();
console.log("1..4");
