import { createNotifyChannelReadClient, NotifyChannelReadClientError } from "./notify-channel-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("NotifyChannelReadClient lists channels with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyChannelReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ channels: [{ id: "feishu-main", type: "feishu", name: "Feishu", enabled: true }] }); } });
  const result = await client.list();
  assertEqual(result.channels[0]?.id, "feishu-main");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/channels");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("NotifyChannelReadClient lists channels with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyChannelReadClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ channels: [] }); } });
  const result = await client.list();
  assertDeepEqual(result, { channels: [] });
  assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/channels");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("NotifyChannelReadClient exposes notify-channel-read nested gateway errors", async () => {
  const client = createNotifyChannelReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_GATEWAY", message: "nested notify channel read failure" } }, { status: 502 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof NotifyChannelReadClientError); assertEqual(error.name, "NotifyClientError"); assertEqual(error.status, 502); assertDeepEqual(error.body, { error: { code: "BAD_GATEWAY", message: "nested notify channel read failure" } }); assertEqual(error.message, "nested notify channel read failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
