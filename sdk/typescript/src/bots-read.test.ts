import { createBotsReadClient, BotsReadClientError } from "./bots-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BotsReadClient lists and gets bots with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsReadClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("detail")) return jsonResponse({ id: "bot-1", name: "reviewer", is_active: true }); return jsonResponse({ bots: [{ id: "bot-1", name: "default" }], total: 1, active: 1 }); } });
  const list = await client.list(); const bot = await client.get("bot-1");
  assertEqual(list.total, 1); assertEqual(bot.name, "reviewer"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/bots"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/bots/detail?id=bot-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("BotsReadClient reads inbox and channel groups with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsReadClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("channels")) return jsonResponse({ groups: [{ id: "g1", channel_type: "telegram" }], count: 1 }); return jsonResponse({ items: [{ id: "in-1", content: "ping" }], count: { unread: 1, total: 1 } }); } });
  const inbox = await client.inbox(true); const groups = await client.channelGroups("telegram");
  assertEqual(inbox.count.unread, 1); assertEqual(groups.groups[0]?.id, "g1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/inbox?unread=true"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/channels/groups?type=telegram"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("BotsReadClient exposes nested read errors", async () => {
  const client = createBotsReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "bot id is required" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof BotsReadClientError); assertEqual(error.name, "BotsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "bot id is required" } }); assertEqual(error.message, "bot id is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
