import { createRealtimeClient } from "./realtime";

function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }

type FakeWebSocketRecord = { url: string; protocols?: string | string[] };
function fakeWebSocketFactory(records: FakeWebSocketRecord[]) {
  return class FakeWebSocket {
    constructor(url: string | URL, protocols?: string | string[]) {
      records.push({ url: String(url), protocols });
    }
  } as unknown as { new (url: string | URL, protocols?: string | string[]): WebSocket };
}

function testRealtimeClientBuildsApiKeyWebSocketUrl() {
  const client = createRealtimeClient({ baseUrl: "http://localhost:9090/", apiKey: "dev-key" });
  const url = client.wsUrl({ query: { session: "s1", debug: true } });
  assertEqual(url, "ws://localhost:9090/v1/ws?session=s1&debug=true&api_key=dev-key");
  console.log("ok - RealtimeClient builds /v1/ws URL with API key query auth");
}

function testRealtimeClientBuildsBearerTokenWebSocketUrl() {
  const client = createRealtimeClient({ baseUrl: "https://agent.test", token: "jwt-token" });
  assertEqual(client.wsUrl(), "wss://agent.test/v1/ws?access_token=jwt-token");
  assertEqual(client.wsUrl({ query: { token: "override" } }), "wss://agent.test/v1/ws?token=override");
  console.log("ok - RealtimeClient builds secure WebSocket URL with token fallback");
}

function testRealtimeClientConnectsAndSendsMessages() {
  const records: FakeWebSocketRecord[] = [];
  const sent: string[] = [];
  const client = createRealtimeClient({ baseUrl: "ws://agent.test", apiKey: "key", WebSocket: fakeWebSocketFactory(records) });
  client.connect({ protocols: ["yunque.v1"] });
  assertEqual(records[0].url, "ws://agent.test/v1/ws?api_key=key");
  assertDeepEqual(records[0].protocols, ["yunque.v1"]);
  client.send({ send: (data) => sent.push(data) }, client.chat("你好", { session: "s1", thinking: true }));
  client.send({ send: (data) => sent.push(data) }, client.ping());
  assertDeepEqual(JSON.parse(sent[0]), { type: "chat", content: "你好", session: "s1", thinking: true });
  assertDeepEqual(JSON.parse(sent[1]), { type: "ping" });
  console.log("ok - RealtimeClient connects and serializes ping/chat messages");
}

function testRealtimeClientParsesMessagesAndRejectsInvalidBaseUrl() {
  const client = createRealtimeClient({ baseUrl: "http://localhost:9090" });
  assertDeepEqual(client.parse('{"type":"pong"}'), { type: "pong" });
  try { client.parse("[]"); throw new Error("expected parse to reject"); } catch (error) { assert(error instanceof Error); assertEqual(error.message, "Realtime message must be an object"); }
  try { createRealtimeClient({ baseUrl: "file:///tmp/socket" }).wsUrl(); throw new Error("expected wsUrl to reject"); } catch (error) { assert(error instanceof Error); assert(error.message.includes("Unsupported realtime baseUrl protocol")); }
  console.log("ok - RealtimeClient parses inbound messages and validates URL protocols");
}

testRealtimeClientBuildsApiKeyWebSocketUrl();
testRealtimeClientBuildsBearerTokenWebSocketUrl();
testRealtimeClientConnectsAndSendsMessages();
testRealtimeClientParsesMessagesAndRejectsInvalidBaseUrl();
console.log("1..4");
