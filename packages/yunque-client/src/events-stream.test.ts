import { createEventsStreamClient, EventsStreamClientError } from "./events-stream";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function streamFromText(text: string): ReadableStream<Uint8Array> { const encoder = new TextEncoder(); return new ReadableStream({ start(controller) { controller.enqueue(encoder.encode(text)); controller.close(); } }); }
async function collect<T>(iterable: AsyncIterable<T>): Promise<T[]> { const out: T[] = []; for await (const item of iterable) out.push(item); return out; }

test("EventsStreamClient streams SSE events with bearer token", async () => {
  const calls: Array<{ url: string; headers: Headers; signal?: AbortSignal | null }> = [];
  const signal = new AbortController().signal;
  const client = createEventsStreamClient({ baseUrl: "https://agent.test/", token: "jwt-token", fetch: async (input, init) => { calls.push({ url: String(input), headers: new Headers(init?.headers), signal: init?.signal }); return new Response(streamFromText('event: connected\ndata: {"client_id":"sse-1"}\n\nevent: task.step_completed\nid: evt-1\ndata: {"task_id":"task-1","step":"scan"}\n\n'), { status: 200, headers: { "Content-Type": "text/event-stream" } }); } });
  const events = await collect(client.stream<Record<string, unknown>>({ signal, headers: { "x-client": "desktop" } }));
  assertEqual(calls[0]?.url, "https://agent.test/v1/events/stream"); assertEqual(calls[0]?.headers.get("authorization"), "Bearer jwt-token"); assertEqual(calls[0]?.headers.get("x-client"), "desktop"); assertEqual(calls[0]?.signal, signal);
  assertEqual(events.length, 2); assertEqual(events[0]?.event, "connected"); assertDeepEqual(events[0]?.data, { client_id: "sse-1" }); assertEqual(events[1]?.id, "evt-1");
});

test("EventsStreamClient supports API key auth and text event payloads", async () => {
  const client = createEventsStreamClient({ baseUrl: "https://agent.test", apiKey: "dev-key", fetch: async (_input, init) => { assertEqual(new Headers(init?.headers).get("x-api-key"), "dev-key"); return new Response(streamFromText("event: notice\ndata: plain text\nretry: 1500\n\n"), { status: 200 }); } });
  const [event] = await collect(client.stream());
  assertEqual(event?.event, "notice"); assertEqual(event?.data, "plain text"); assertEqual(event?.retry, 1500);
});

test("EventsStreamClient exposes nested stream errors through alias", async () => {
  const client = createEventsStreamClient({ baseUrl: "https://agent.test", fetch: async () => new Response(JSON.stringify({ error: { code: "BAD_REQUEST", message: "event stream tenant is required" } }), { status: 400 }) });
  try { await collect(client.stream()); throw new Error("expected stream to reject"); } catch (error) { assert(error instanceof EventsStreamClientError); assertEqual(error.name, "EventsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "event stream tenant is required" } }); assertEqual(error.message, "event stream tenant is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
