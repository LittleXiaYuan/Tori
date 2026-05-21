import { createBotsInboxClient, BotsInboxClientError } from "./bots-inbox";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BotsInboxClient lists and pushes inbox with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsInboxClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "in-2", content: "pong", action: "trigger" }, { status: 201 }); return jsonResponse({ items: [{ id: "in-1", content: "ping" }], count: { unread: 1, total: 1 } }); } });
  const inbox = await client.list(true); const item = await client.push({ source: "webhook", content: "pong", action: "trigger" });
  assertEqual(inbox.count.unread, 1); assertEqual(item.action, "trigger"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/inbox?unread=true"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { source: "webhook", content: "pong", action: "trigger", header: {} });
});

test("BotsInboxClient marks and deletes inbox with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsInboxClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/inbox/read")) return jsonResponse({ marked: 2 }); return jsonResponse({ status: "ok" }); } });
  const marked = await client.markRead(["in-1", "in-2"]); const all = await client.markAllRead(); const deleted = await client.delete("in-1");
  assertEqual(marked.marked, 2); assertEqual(all.marked, 2); assertEqual(deleted.status, "ok"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { ids: ["in-1", "in-2"], all: false }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { all: true }); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { id: "in-1" });
});

test("BotsInboxClient exposes nested inbox errors", async () => {
  const client = createBotsInboxClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested inbox failed" } }, { status: 400 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof BotsInboxClientError); assertEqual(error.name, "BotsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested inbox failed" } }); assertEqual(error.message, "nested inbox failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
