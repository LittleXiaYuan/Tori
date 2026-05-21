import { createBotsControlClient, BotsControlClientError } from "./bots-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BotsControlClient creates updates and deletes bots with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ status: "ok" }); if (init?.method === "PUT") return jsonResponse({ id: "bot-1", name: "reviewer", is_active: false }); return jsonResponse({ id: "bot-2", name: "planner", is_active: true }, { status: 201 }); } });
  const created = await client.create({ name: "planner", description: "计划", config: { model: "deepseek", reasoning_enabled: true } }); const updated = await client.setActive("bot-1", false); const deleted = await client.delete("bot-1");
  assertEqual(created.name, "planner"); assertEqual(updated.is_active, false); assertEqual(deleted.status, "ok"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "planner", description: "计划", config: { model: "deepseek", reasoning_enabled: true } }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { active: false }); assertEqual(calls[2]?.url, "http://localhost:9090/v1/bots/detail?id=bot-1");
});

test("BotsControlClient manages inbox items with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsControlClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/inbox/read")) return jsonResponse({ marked: 2 }); if (init?.method === "DELETE") return jsonResponse({ status: "ok" }); return jsonResponse({ id: "in-1", content: "ping", action: "trigger" }, { status: 201 }); } });
  const item = await client.pushInbox({ source: "webhook", content: "ping", action: "trigger" }); const marked = await client.markInboxRead(["in-1", "in-2"]); const all = await client.markAllInboxRead(); const deleted = await client.deleteInbox("in-1");
  assertEqual(item.action, "trigger"); assertEqual(marked.marked, 2); assertEqual(all.marked, 2); assertEqual(deleted.status, "ok"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { ids: ["in-1", "in-2"], all: false }); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { all: true }); assertDeepEqual(JSON.parse(String(calls[3]?.init?.body)), { id: "in-1" });
});

test("BotsControlClient exposes text control errors", async () => {
  const client = createBotsControlClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET or POST", { status: 405 }) });
  try { await client.create({ name: "x" }); throw new Error("expected create to reject"); } catch (error) { assert(error instanceof BotsControlClientError); assertEqual(error.name, "BotsClientError"); assertEqual(error.status, 405); assertEqual(error.body, "GET or POST"); assertEqual(error.message, "GET or POST"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
