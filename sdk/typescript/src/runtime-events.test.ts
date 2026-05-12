import { createRuntimeEventsClient, RuntimeEventsClientError } from "./runtime-events";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
function sseResponse(chunks: string[]): Response { return new Response(new ReadableStream<Uint8Array>({ start(controller) { const encoder = new TextEncoder(); for (const chunk of chunks) controller.enqueue(encoder.encode(chunk)); controller.close(); } }), { status: 200, headers: { "Content-Type": "text/event-stream" } }); }

test("RuntimeEventsClient streams events with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeEventsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return sseResponse(['event: connected\ndata: {"client_id":"sse-1"}\n\n', 'event: task.done\nid: evt-1\ndata: {"task_id":"task-1"}\n\n']); } });
  const events = [];
  for await (const event of client.events()) events.push(event);
  assertEqual(events.length, 2);
  assertEqual(events[0]?.event, "connected");
  assertEqual(events[1]?.id, "evt-1");
  assertDeepEqual(events[1]?.data, { task_id: "task-1" });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/events/stream");
  const headers = new Headers(calls[0]?.init?.headers);
  assertEqual(headers.get("authorization"), "Bearer jwt");
  assertEqual(headers.get("accept"), "text/event-stream");
});

test("RuntimeEventsClient supports API key auth and multiline data", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRuntimeEventsClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return sseResponse(['event: message\ndata: line one\ndata: line two\n\n', ': keepalive\n\n']); } });
  const events = [];
  for await (const event of client.events()) events.push(event);
  assertEqual(events.length, 1);
  assertEqual(events[0]?.type, "message");
  assertEqual(events[0]?.data, "line one\nline two");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("RuntimeEventsClient exposes nested event stream errors", async () => {
  const client = createRuntimeEventsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "EVENTS_DOWN", message: "runtime events unavailable" } }, { status: 503 }) });
  try { for await (const _event of client.events()) { /* no-op */ } throw new Error("expected events to reject"); } catch (error) { assert(error instanceof RuntimeEventsClientError); assertEqual(error.name, "RuntimeClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "EVENTS_DOWN", message: "runtime events unavailable" } }); assertEqual(error.message, "runtime events unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
