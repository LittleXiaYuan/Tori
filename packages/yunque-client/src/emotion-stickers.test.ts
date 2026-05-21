import { createEmotionStickersClient, EmotionStickersClientError } from "./emotion-stickers";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("EmotionStickersClient lists sticker mappings with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createEmotionStickersClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ telegram: { happy: [{ package_id: "pkg", sticker_id: "stk" }] } }); } });
  const stickers = await client.list();
  assertEqual(stickers.telegram?.happy?.[0]?.sticker_id, "stk"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/emotion/stickers"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("EmotionStickersClient registers and clears mappings with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createEmotionStickersClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.register({ platform: "telegram", emotion: "happy", stickers: [{ package_id: "p1", sticker_id: "s1" }] }); await client.clear({ platform: "telegram", emotion: "happy" });
  assertEqual(calls[0]?.init?.method, "PUT"); assertEqual(calls[1]?.init?.method, "DELETE"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { platform: "telegram", emotion: "happy", stickers: [{ package_id: "p1", sticker_id: "s1" }] });
});

test("EmotionStickersClient exposes nested sticker errors", async () => {
  const client = createEmotionStickersClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "platform is required" } }, { status: 400 }) });
  try { await client.register({ platform: "", emotion: "happy", stickers: [] }); throw new Error("expected register to reject"); } catch (error) { assert(error instanceof EmotionStickersClientError); assertEqual(error.name, "InteractionsClientError"); assertEqual(error.status, 400); assertEqual(error.message, "platform is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
