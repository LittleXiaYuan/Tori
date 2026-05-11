import { createReactionsClient, ReactionsClientError } from "./reactions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ReactionsClient sends reactions through reactions facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReactionsClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });

  const result = await client.react({ channel_type: "wechat", target: "u1", message_id: "m1", emoji: "👍" });

  assertEqual(result.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/react");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ channel_type: "wechat", target: "u1", message_id: "m1", emoji: "👍" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ReactionsClient sends stickers without generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReactionsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "sent" }); } });

  const result = await client.sendSticker({ channel_type: "wechat", target: "u1", emoji: "🌟" });

  assertEqual(result.status, "sent");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/sticker/send");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ channel_type: "wechat", target: "u1", emoji: "🌟" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ReactionsClient exposes reactions-named errors", async () => {
  const client = createReactionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "reaction unavailable" } }, { status: 503 }) });

  try {
    await client.react({ channel_type: "wechat", target: "u1", message_id: "m1" });
    throw new Error("expected react to reject");
  } catch (error) {
    assert(error instanceof ReactionsClientError);
    assertEqual(error.status, 503);
    assertEqual(error.message, "reaction unavailable");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
