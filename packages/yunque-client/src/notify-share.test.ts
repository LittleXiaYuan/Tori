import { createNotifyShareClient, NotifyShareClientError } from "./notify-share";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("NotifyShareClient shares chat artifacts with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyShareClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, sent_at: "2026-05-12T00:00:00Z", share: { code: "yq_abc", session_id: "s1", created_at: "2026-05-12T00:00:00Z" }, channel: { id: "c1", type: "feishu", name: "Feishu" } }); } });
  const result = await client.send({ channel_id: "c1", title: "复盘", message: "已完成", session_id: "s1", files: [{ name: "report.md", path: "out/report.md", size: 12 }] });
  assertEqual(result.share?.code, "yq_abc");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/share");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { channel_id: "c1", title: "复盘", message: "已完成", session_id: "s1", files: [{ name: "report.md", path: "out/report.md", size: 12 }] });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("NotifyShareClient supports task and url payloads with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createNotifyShareClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, sent_at: "2026-05-12T00:00:00Z" }); } });
  const result = await client.send({ channel_id: "c2", title: "任务完成", task_id: "task-1", url: "http://localhost:9090/tasks/task-1" });
  assertEqual(result.ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/notify/share");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { channel_id: "c2", title: "任务完成", task_id: "task-1", url: "http://localhost:9090/tasks/task-1" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("NotifyShareClient exposes notify-share nested gateway errors", async () => {
  const client = createNotifyShareClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested notify share failure" } }, { status: 400 }) });
  try { await client.send({ channel_id: "" }); throw new Error("expected send to reject"); } catch (error) { assert(error instanceof NotifyShareClientError); assertEqual(error.name, "NotifyClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested notify share failure" } }); assertEqual(error.message, "nested notify share failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
