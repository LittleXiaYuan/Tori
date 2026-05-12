import { createEventsParseClient } from "./events-parse";

declare const process: { exitCode?: number };
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function streamFromText(text: string): ReadableStream<Uint8Array> { const encoder = new TextEncoder(); return new ReadableStream({ start(controller) { controller.enqueue(encoder.encode(text)); controller.close(); } }); }
async function collect<T>(iterable: AsyncIterable<T>): Promise<T[]> { const out: T[] = []; for await (const item of iterable) out.push(item); return out; }

test("EventsParseClient parses multiline and default SSE frames", async () => {
  const client = createEventsParseClient({ baseUrl: "https://agent.test", fetch: async () => new Response() });
  const events = await collect(client.parseStream(streamFromText("data: first\ndata: second\n\nid: only-id\n\n")));
  assertEqual(events[0]?.event, "message"); assertEqual(events[0]?.data, "first\nsecond"); assertEqual(events[1]?.event, "message"); assertEqual(events[1]?.id, "only-id"); assertEqual(events[1]?.data, undefined);
});

test("EventsParseClient parses JSON, text, id and retry fields", async () => {
  const client = createEventsParseClient({ baseUrl: "https://agent.test", fetch: async () => new Response() });
  const events = await collect(client.parseStream(streamFromText('event: task\nid: evt-2\nretry: 2500\ndata: {"ok":true}\n\nevent: notice\ndata: plain text\n\n')));
  assertEqual(events[0]?.event, "task"); assertEqual(events[0]?.id, "evt-2"); assertEqual(events[0]?.retry, 2500); assertDeepEqual(events[0]?.data, { ok: true }); assertEqual(events[1]?.data, "plain text");
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
