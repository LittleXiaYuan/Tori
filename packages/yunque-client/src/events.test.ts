import { createEventsClient, EventsClientError } from "./events";

function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
async function assertRejects(fn: () => Promise<unknown>, check: (error: unknown) => void): Promise<void> { try { await fn(); } catch (error) { check(error); return; } throw new Error("expected promise to reject"); }

function streamFromText(text: string): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream({
    start(controller) {
      controller.enqueue(encoder.encode(text));
      controller.close();
    },
  });
}

async function collect<T>(iterable: AsyncIterable<T>): Promise<T[]> {
  const out: T[] = [];
  for await (const item of iterable) out.push(item);
  return out;
}

async function testEventsClientStreamsSSEWithBearerToken() {
  const requests: Array<{ url: string; headers: Headers }> = [];
  const client = createEventsClient({
    baseUrl: "https://agent.test/",
    token: "jwt-token",
    fetch: async (input, init) => {
      requests.push({ url: String(input), headers: new Headers(init?.headers) });
      return new Response(
        streamFromText([
          'event: connected',
          'data: {"client_id":"sse-1"}',
          '',
          ': keepalive ignored',
          '',
          'event: task.step_completed',
          'id: evt-1',
          'data: {"task_id":"task-1","step":"scan"}',
          '',
        ].join('\n')),
        { status: 200, headers: { "Content-Type": "text/event-stream" } },
      );
    },
  });

  const events = await collect(client.stream<Record<string, unknown>>());
  assertEqual(requests[0].url, "https://agent.test/v1/events/stream");
  assertEqual(requests[0].headers.get("authorization"), "Bearer jwt-token");
  assertEqual(events.length, 2);
  assertEqual(events[0].event, "connected");
  assertDeepEqual(events[0].data, { client_id: "sse-1" });
  assertEqual(events[1].event, "task.step_completed");
  assertEqual(events[1].id, "evt-1");
  assertDeepEqual(events[1].data, { task_id: "task-1", step: "scan" });
  console.log("ok - EventsClient streams SSE events with bearer token");
}

async function testEventsClientSupportsApiKeyAndTextPayloads() {
  const client = createEventsClient({
    baseUrl: "https://agent.test",
    apiKey: "dev-key",
    fetch: async (_input, init) => {
      const headers = new Headers(init?.headers);
      assertEqual(headers.get("x-api-key"), "dev-key");
      return new Response(streamFromText("event: notice\ndata: plain text\nretry: 1500\n\n"), { status: 200 });
    },
  });

  const [event] = await collect(client.stream());
  assertEqual(event.event, "notice");
  assertEqual(event.data, "plain text");
  assertEqual(event.retry, 1500);
  console.log("ok - EventsClient supports API key auth and text event payloads");
}

async function testEventsClientParsesMultilineAndDefaultEvents() {
  const client = createEventsClient({ baseUrl: "https://agent.test", fetch: async () => new Response() });
  const events = await collect(client.parseStream(streamFromText("data: first\ndata: second\n\nid: only-id\n\n")));
  assertEqual(events[0].event, "message");
  assertEqual(events[0].data, "first\nsecond");
  assertEqual(events[1].event, "message");
  assertEqual(events[1].id, "only-id");
  assertEqual(events[1].data, undefined);
  console.log("ok - EventsClient parses multiline and default SSE frames");
}

async function testEventsClientThrowsParsedAndTextErrors() {
  const jsonClient = createEventsClient({
    baseUrl: "https://agent.test",
    fetch: async () => new Response(JSON.stringify({ error: "SSE not available" }), { status: 503 }),
  });
  await assertRejects(() => collect(jsonClient.stream()), (err: unknown) => {
    assert(err instanceof EventsClientError);
    assertEqual(err.status, 503);
    assertDeepEqual(err.body, { error: "SSE not available" });
    assertEqual(err.message, "SSE not available");
  });

  const nestedClient = createEventsClient({
    baseUrl: "https://agent.test",
    fetch: async () => new Response(JSON.stringify({ error: { code: "BAD_REQUEST", message: "event stream tenant is required" } }), { status: 400 }),
  });
  await assertRejects(() => collect(nestedClient.stream()), (err: unknown) => {
    assert(err instanceof EventsClientError);
    assertEqual(err.status, 400);
    assertEqual(err.message, "event stream tenant is required");
  });

  const textClient = createEventsClient({
    baseUrl: "https://agent.test",
    fetch: async () => new Response("streaming not supported", { status: 500 }),
  });
  await assertRejects(() => collect(textClient.stream()), (err: unknown) => {
    assert(err instanceof EventsClientError);
    assertEqual(err.status, 500);
    assertEqual(err.body, "streaming not supported");
    assertEqual(err.message, "streaming not supported");
  });
  console.log("ok - EventsClient throws EventsClientError with parsed and text bodies");
}

await testEventsClientStreamsSSEWithBearerToken();
await testEventsClientSupportsApiKeyAndTextPayloads();
await testEventsClientParsesMultilineAndDefaultEvents();
await testEventsClientThrowsParsedAndTextErrors();
console.log("1..4");
