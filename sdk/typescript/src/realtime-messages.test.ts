import { createRealtimeMessagesClient } from "./realtime-messages";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }

test("RealtimeMessagesClient serializes ping and chat messages", () => {
  const sent: string[] = [];
  const client = createRealtimeMessagesClient({ baseUrl: "http://localhost:9090" });
  client.send({ send: (data) => sent.push(data) }, client.chat("你好", { session: "s1", thinking: true }));
  client.send({ send: (data) => sent.push(data) }, client.ping({ nonce: 1 }));
  assertDeepEqual(JSON.parse(sent[0] || "{}"), { type: "chat", content: "你好", session: "s1", thinking: true });
  assertDeepEqual(JSON.parse(sent[1] || "{}"), { type: "ping", nonce: 1 });
});

test("RealtimeMessagesClient parses inbound objects and rejects invalid payloads", () => {
  const client = createRealtimeMessagesClient({ baseUrl: "http://localhost:9090" });
  assertDeepEqual(client.parse('{"type":"pong","session":"s1"}'), { type: "pong", session: "s1" });
  try { client.parse("[]"); throw new Error("expected parse to reject"); } catch (error) { assert(error instanceof Error); assertEqual(error.message, "Realtime message must be an object"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
