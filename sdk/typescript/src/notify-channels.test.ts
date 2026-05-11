import { createNotifyChannelsClient, NotifyChannelsClientError } from "./notify-channels";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("NotifyChannelsClient lists and adds channels with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyChannelsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ ok: true }); return jsonResponse({ channels: [{ id: "feishu-main", type: "feishu", name: "Feishu", enabled: true }] }); } });
  const list = await client.list();
  const added = await client.add({ id: "feishu-main", type: "feishu", name: "Feishu", url: "https://hook" });
  assertEqual(list.channels[0]?.id, "feishu-main");
  assertEqual(added.ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/channels");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "feishu-main", type: "feishu", name: "Feishu", url: "https://hook" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("NotifyChannelsClient removes toggles and tests channels with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyChannelsClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true }); } });
  await client.remove("c1");
  await client.toggle({ id: "c1", enabled: false });
  await client.test("c1");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/remove?id=c1");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/notify/toggle");
  assertEqual(calls[2]?.url, "http://localhost:9090/api/notify/test");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "c1", enabled: false });
  assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { id: "c1" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("NotifyChannelsClient exposes notify-channels nested gateway errors", async () => {
  const client = createNotifyChannelsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested notify channel failure" } }, { status: 400 }) });
  try { await client.add({ id: "", type: "webhook", name: "" }); throw new Error("expected add to reject"); } catch (error) { assert(error instanceof NotifyChannelsClientError); assertEqual(error.name, "NotifyClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested notify channel failure" } }); assertEqual(error.message, "nested notify channel failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
