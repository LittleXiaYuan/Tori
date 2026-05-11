import { createBotsClient, BotsClientError } from "./bots";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BotsClient lists and creates bots with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "bot-2", name: "planner", is_active: true }, { status: 201 }); return jsonResponse({ bots: [{ id: "bot-1", name: "default" }], total: 1, active: 1 }); } });
  const list = await client.list(); const created = await client.create({ name: "planner", description: "计划", config: { model: "deepseek", reasoning_enabled: true } });
  assertEqual(list.total, 1); assertEqual(created.name, "planner"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { name: "planner", description: "计划", config: { model: "deepseek", reasoning_enabled: true } });
});

test("BotsClient manages bot detail with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ status: "ok" }); if (init?.method === "PUT") return jsonResponse({ id: "bot-1", name: "reviewer", is_active: false }); return jsonResponse({ id: "bot-1", name: "reviewer", is_active: true }); } });
  const bot = await client.get("bot-1"); const updated = await client.setActive("bot-1", false); const deleted = await client.delete("bot-1");
  assertEqual(bot.name, "reviewer"); assertEqual(updated.is_active, false); assertEqual(deleted.status, "ok"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/bots/detail?id=bot-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { active: false });
});

test("BotsClient manages inbox items", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/inbox/read")) return jsonResponse({ marked: 2 }); if (init?.method === "POST") return jsonResponse({ id: "in-1", content: "ping", action: "trigger" }, { status: 201 }); if (init?.method === "DELETE") return jsonResponse({ status: "ok" }); return jsonResponse({ items: [{ id: "in-1", content: "ping" }], count: { unread: 1, total: 1 } }); } });
  const inbox = await client.inbox(true); const item = await client.pushInbox({ source: "webhook", content: "ping", action: "trigger" }); const marked = await client.markInboxRead(["in-1", "in-2"]); const deleted = await client.deleteInbox("in-1");
  assertEqual(inbox.count.unread, 1); assertEqual(item.action, "trigger"); assertEqual(marked.marked, 2); assertEqual(deleted.status, "ok"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/inbox?unread=true"); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { ids: ["in-1", "in-2"], all: false });
});

test("BotsClient reads channel groups and marks all inbox read", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("channels")) return jsonResponse({ groups: [{ id: "g1", channel_type: "telegram" }], count: 1 }); return jsonResponse({ marked: 3 }); } });
  const groups = await client.channelGroups("telegram"); const marked = await client.markAllInboxRead();
  assertEqual(groups.groups[0]?.id, "g1"); assertEqual(marked.marked, 3); assertEqual(calls[0]?.url, "http://localhost:9090/v1/channels/groups?type=telegram"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { all: true });
});

test("BotsClient throws BotsClientError with parsed and text bodies", async () => {
  const jsonClient = createBotsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "bot not found" }, { status: 404 }) });
  try { await jsonClient.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof BotsClientError); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: "bot not found" }); assertEqual(error.message, "bot not found"); }
  const textClient = createBotsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET or POST", { status: 405 }) });
  try { await textClient.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof BotsClientError); assertEqual(error.status, 405); assertEqual(error.body, "GET or POST"); assertEqual(error.message, "GET or POST"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);

