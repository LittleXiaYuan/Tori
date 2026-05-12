import { createRealtimeConnectClient } from "./realtime-connect";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }

type FakeWebSocketRecord = { url: string; protocols?: string | string[] };
function fakeWebSocketFactory(records: FakeWebSocketRecord[]) { return class FakeWebSocket { constructor(url: string | URL, protocols?: string | string[]) { records.push({ url: String(url), protocols }); } } as unknown as { new (url: string | URL, protocols?: string | string[]): WebSocket }; }

test("RealtimeConnectClient builds WebSocket URLs with API key and token fallback", () => {
  const apiKeyClient = createRealtimeConnectClient({ baseUrl: "http://localhost:9090/", apiKey: "dev-key" });
  assertEqual(apiKeyClient.wsUrl({ query: { session: "s1", debug: true } }), "ws://localhost:9090/v1/ws?session=s1&debug=true&api_key=dev-key");
  const tokenClient = createRealtimeConnectClient({ baseUrl: "https://agent.test", token: "jwt-token" });
  assertEqual(tokenClient.wsUrl(), "wss://agent.test/v1/ws?access_token=jwt-token");
  assertEqual(tokenClient.wsUrl({ query: { token: "override" } }), "wss://agent.test/v1/ws?token=override");
});

test("RealtimeConnectClient opens sockets with protocols and validates baseUrl", () => {
  const records: FakeWebSocketRecord[] = [];
  const client = createRealtimeConnectClient({ baseUrl: "ws://agent.test", apiKey: "key", WebSocket: fakeWebSocketFactory(records) });
  client.connect({ protocols: ["yunque.v1"] });
  assertEqual(records[0]?.url, "ws://agent.test/v1/ws?api_key=key"); assertDeepEqual(records[0]?.protocols, ["yunque.v1"]);
  try { createRealtimeConnectClient({ baseUrl: "file:///tmp/socket" }).wsUrl(); throw new Error("expected wsUrl to reject"); } catch (error) { assert(error instanceof Error); assert(error.message.includes("Unsupported realtime baseUrl protocol")); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
