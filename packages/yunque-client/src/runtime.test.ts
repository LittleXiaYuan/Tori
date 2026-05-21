import { createRuntimeClient, RuntimeClientError } from "./runtime";

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

function sseResponse(chunks: string[]): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      const encoder = new TextEncoder();
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
      controller.close();
    },
  });
  return new Response(stream, {
    status: 200,
    headers: { "Content-Type": "text/event-stream" },
  });
}

test("RuntimeClient reads queue overview with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ queues: { "session-1": 2 } });
    },
  });

  const result = await client.queues();

  assertEqual(result.queues?.["session-1"], 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sessions/queue");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("RuntimeClient reads session queue and cancels queued task", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("/cancel")) return jsonResponse({ cancelled: true });
      return jsonResponse({ session_id: "session/1", tasks: [{ task_id: "task-1", status: "queued" }] });
    },
  });

  const queue = await client.sessionQueue("session/1");
  const cancelled = await client.cancelQueuedTask("session/1", "task-1");

  assertEqual(queue.tasks[0]?.task_id, "task-1");
  assertEqual(cancelled.cancelled, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sessions/queue?id=session%2F1");
  assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ session_id: "session/1", task_id: "task-1" }));
});

test("RuntimeClient parses events stream", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return sseResponse([
        'event: connected\ndata: {"client_id":"sse-1"}\n\n',
        'event: task.step_completed\ndata: {"task_id":"task-1","step":2}\n\n',
        ': keepalive 2026-05-11T00:00:00Z\n\n',
      ]);
    },
  });

  const events = [];
  for await (const event of client.events()) events.push(event);

  assertEqual(events.length, 2);
  assertEqual(events[0]?.event, "connected");
  assertDeepEqual(events[1]?.data, { task_id: "task-1", step: 2 });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/events/stream");
  assertEqual(new Headers(calls[0]?.init?.headers).get("accept"), "text/event-stream");
});

test("RuntimeClient throws RuntimeClientError with text body", async () => {
  const client = createRuntimeClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("SSE not available", { status: 503 }),
  });

  try {
    for await (const _event of client.events()) {
      // no-op
    }
    throw new Error("expected events to reject");
  } catch (error) {
    assert(error instanceof RuntimeClientError);
    assertEqual(error.status, 503);
    assertDeepEqual(error.body, "SSE not available");
    assertEqual(error.message, "SSE not available");
  }

  const nestedClient = createRuntimeClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "runtime session id is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.cancelQueuedTask("", "task-1");
    throw new Error("expected cancelQueuedTask to reject");
  } catch (error) {
    assert(error instanceof RuntimeClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "runtime session id is required");
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
