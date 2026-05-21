import { createInteractionsClient, InteractionsClientError } from "./interactions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("InteractionsClient reads emotion history and sticker mappings with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createInteractionsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("history")) return jsonResponse({ entries: [{ emotion: "happy" }], summary: { happy: 1 }, total: 1 }); return jsonResponse({ telegram: { happy: [{ package_id: "pkg", sticker_id: "stk" }] } }); } });
  const history = await client.emotionHistory({ sessionId: "s1", limit: 5, from: "2026-05-11T00:00:00Z" }); const stickers = await client.stickers();
  assertEqual(history.total, 1); assertEqual(stickers.telegram?.happy?.[0]?.sticker_id, "stk"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/emotion/history?session_id=s1&limit=5&from=2026-05-11T00%3A00%3A00Z"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("InteractionsClient manages sticker mappings with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createInteractionsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.registerStickers({ platform: "telegram", emotion: "happy", stickers: [{ package_id: "p1", sticker_id: "s1" }] }); await client.clearStickers({ platform: "telegram", emotion: "happy" });
  assertEqual(calls[0]?.init?.method, "PUT"); assertEqual(calls[1]?.init?.method, "DELETE"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { platform: "telegram", emotion: "happy", stickers: [{ package_id: "p1", sticker_id: "s1" }] });
});

test("InteractionsClient manages user instructions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createInteractionsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("reorder")) return jsonResponse({ status: "reordered" }); if (init?.method === "POST") return jsonResponse({ instruction_id: "i1", content: "保持简洁", category: "style" }, { status: 201 }); if (init?.method === "PUT") return jsonResponse({ status: "updated" }); if (init?.method === "DELETE") return jsonResponse({ status: "deleted" }); return jsonResponse({ instructions: [{ instruction_id: "i1", content: "保持简洁" }], total: 1 }); } });
  const list = await client.instructions("style"); const created = await client.createInstruction({ category: "style", content: "保持简洁" }); const updated = await client.updateInstruction({ instruction_id: "i1", content: "更自然" }); const reordered = await client.reorderInstructions(["i1"]); const deleted = await client.deleteInstruction("i1");
  assertEqual(list.total, 1); assertEqual(created.instruction_id, "i1"); assertEqual(updated.status, "updated"); assertEqual(reordered.status, "reordered"); assertEqual(deleted.status, "deleted"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/instructions?category=style");
});

test("InteractionsClient sends reactions and stickers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createInteractionsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.react({ channel_type: "telegram", target: "chat-1", message_id: "m1", emoji: "👍" }); await client.sendSticker({ channel_type: "telegram", target: "chat-1", package_id: "p1", sticker_id: "s1" });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/react"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/sticker/send"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { channel_type: "telegram", target: "chat-1", message_id: "m1", emoji: "👍" });
});

test("InteractionsClient throws InteractionsClientError with parsed and text bodies", async () => {
  const jsonClient = createInteractionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "id query parameter required" }, { status: 400 }) });
  try { await jsonClient.deleteInstruction(""); throw new Error("expected deleteInstruction to reject"); } catch (error) { assert(error instanceof InteractionsClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "id query parameter required" }); assertEqual(error.message, "id query parameter required"); }
  const nestedClient = createInteractionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "instruction content is required" } }, { status: 400 }) });
  try { await nestedClient.createInstruction({ category: "style", content: "" }); throw new Error("expected createInstruction to reject"); } catch (error) { assert(error instanceof InteractionsClientError); assertEqual(error.status, 400); assertEqual(error.message, "instruction content is required"); }
  const textClient = createInteractionsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.emotionHistory(); throw new Error("expected emotionHistory to reject"); } catch (error) { assert(error instanceof InteractionsClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
