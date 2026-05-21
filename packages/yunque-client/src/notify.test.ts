import { createNotifyClient, NotifyClientError } from "./notify";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("NotifyClient lists and adds channels with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ ok: true }); return jsonResponse({ channels: [{ id: "feishu-main", type: "feishu", name: "Feishu", enabled: true, url: "https://***" }] }); } });
  const list = await client.channels(); const added = await client.addChannel({ id: "feishu-main", type: "feishu", name: "Feishu", url: "https://hook" });
  assertEqual(list.channels[0]?.id, "feishu-main"); assertEqual(added.ok, true); assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/channels"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "feishu-main", type: "feishu", name: "Feishu", url: "https://hook" });
});

test("NotifyClient removes toggles and tests channels with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true }); } });
  await client.removeChannel("c1"); await client.toggleChannel({ id: "c1", enabled: false }); await client.testChannel("c1");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/remove?id=c1"); assertEqual(calls[1]?.url, "http://localhost:9090/api/notify/toggle"); assertEqual(calls[2]?.url, "http://localhost:9090/api/notify/test"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "c1", enabled: false }); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { id: "c1" });
});

test("NotifyClient shares chat artifacts to a channel", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, sent_at: "2026-05-12T00:00:00Z", share: { code: "yq_abc", session_id: "s1", created_at: "2026-05-12T00:00:00Z" }, channel: { id: "c1", type: "feishu", name: "Feishu" } }); } });
  const shared = await client.share({ channel_id: "c1", title: "复盘", message: "已完成", session_id: "s1", files: [{ name: "report.md", path: "out/report.md", size: 12 }] });
  assertEqual(shared.share?.code, "yq_abc"); assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/share"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { channel_id: "c1", title: "复盘", message: "已完成", session_id: "s1", files: [{ name: "report.md", path: "out/report.md", size: 12 }] });
});

test("NotifyClient throws NotifyClientError with parsed and text bodies", async () => {
  const jsonClient = createNotifyClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "channel_id required" }, { status: 400 }) });
  try { await jsonClient.share({ channel_id: "" }); throw new Error("expected share to reject"); } catch (error) { assert(error instanceof NotifyClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "channel_id required" }); assertEqual(error.message, "channel_id required"); }
  const nestedClient = createNotifyClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested channel_id required" } }, { status: 400 }) });
  try { await nestedClient.share({ channel_id: "" }); throw new Error("expected nested share to reject"); } catch (error) { assert(error instanceof NotifyClientError); assertEqual(error.status, 400); assertEqual(error.message, "nested channel_id required"); }
  const textClient = createNotifyClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST required", { status: 405 }) });
  try { await textClient.addChannel({ id: "", type: "webhook", name: "" }); throw new Error("expected addChannel to reject"); } catch (error) { assert(error instanceof NotifyClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST required"); assertEqual(error.message, "POST required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
