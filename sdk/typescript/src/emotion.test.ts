import { createEmotionClient, EmotionClientError } from "./emotion";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("EmotionClient reads emotion history through emotion facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createEmotionClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entries: [{ emotion: "happy" }], total: 1 }); } });

  const result = await client.history({ sessionId: "s1", limit: 5 });

  assertEqual(result.total, 1);
  assertEqual(result.entries[0]?.emotion, "happy");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/emotion/history?session_id=s1&limit=5");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("EmotionClient manages stickers without generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createEmotionClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (!init || init.method === "GET") return jsonResponse({ happy: { wechat: [{ package_id: "p1", sticker_id: "s1" }] } }); return jsonResponse({ status: "ok" }); } });

  const stickers = await client.stickers();
  await client.registerStickers({ platform: "wechat", emotion: "happy", stickers: [{ package_id: "p1", sticker_id: "s1" }] });
  await client.clearStickers({ platform: "wechat", emotion: "happy" });

  assertEqual(stickers.happy?.wechat?.[0]?.sticker_id, "s1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/emotion/stickers");
  assertEqual(calls[1]?.init?.method, "PUT");
  assertEqual(calls[2]?.init?.method, "DELETE");
  assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "key-123");
});

test("EmotionClient exposes emotion-named errors", async () => {
  const client = createEmotionClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "emotion unavailable" } }, { status: 503 }) });

  try {
    await client.history();
    throw new Error("expected history to reject");
  } catch (error) {
    assert(error instanceof EmotionClientError);
    assertEqual(error.status, 503);
    assertEqual(error.message, "emotion unavailable");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
