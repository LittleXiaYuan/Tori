import { createChatClient, ChatClientError } from "./chat";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
}

const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];

function test(name: string, fn: () => Promise<void> | void): void {
  tests.push({ name, fn });
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

function streamFromText(text: string): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoder.encode(text));
      controller.close();
    },
  });
}

test("ChatClient sends chat messages with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createChatClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ reply: "你好", skills_used: [], steps: 1 });
    },
  });

  const result = await client.send({ messages: [{ role: "user", content: "你好" }], session_id: "s1" });

  assertEqual(result.reply, "你好");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/chat");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ messages: [{ role: "user", content: "你好" }], session_id: "s1" }));
});

test("ChatClient supports API key auth for lightweight embedder backends", async () => {
  const calls: { init?: RequestInit }[] = [];
  const client = createChatClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      calls.push({ init });
      return jsonResponse({ reply: "ok" });
    },
  });

  await client.agentic({ messages: [{ role: "user", content: "run" }] });

  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ChatClient parses SSE delta and done frames", async () => {
  const client = createChatClient({
    baseUrl: "http://localhost:9090",
    fetch: async () =>
      new Response(
        streamFromText('data: {"content":"你"}\n\nevent: done\ndata: {"steps":1,"skills_used":[]}\n\ndata: [DONE]\n\n'),
        { status: 200, headers: { "Content-Type": "text/event-stream" } },
      ),
  });

  const items = [];
  for await (const item of client.stream({ messages: [{ role: "user", content: "hello" }] })) items.push(item);

  assertDeepEqual(items, [
    { kind: "delta", content: "你" },
    { kind: "done", data: { steps: 1, skills_used: [] }, raw: '{"steps":1,"skills_used":[]}' },
  ]);
});

test("ChatClient throws ChatClientError with parsed body", async () => {
  const client = createChatClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "unauthorized" }, { status: 401 }),
  });

  try {
    await client.send({ messages: [{ role: "user", content: "hello" }] });
    throw new Error("expected send to reject");
  } catch (error) {
    assert(error instanceof ChatClientError);
    assertEqual(error.status, 401);
    assertDeepEqual(error.body, { error: "unauthorized" });
    assertEqual(error.message, "unauthorized");
  }
});

let failures = 0;
for (const { name, fn } of tests) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failures > 0) {
  process.exitCode = 1;
} else {
  console.log(`1..${tests.length}`);
}
