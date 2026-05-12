import { createPluginCronControlClient, PluginCronControlClientError } from "./plugin-cron-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginCronControlClient adds and removes plugin cron jobs with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginCronControlClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/cron/add")) return jsonResponse({ id: "cron-1", status: "created" }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.add("daily", "0 8 * * *", "ping")).id, "cron-1");
  assertEqual((await client.remove("cron-1")).ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/cron/add");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "daily", expression: "0 8 * * *", message: "ping" });
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/plugin-api/cron/remove");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "cron-1" });
});

test("PluginCronControlClient exposes plugin-cron-control nested gateway errors", async () => {
  const client = createPluginCronControlClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_CRON_CONTROL", message: "nested plugin cron control failure" } }, { status: 403 }) });
  try { await client.add("daily", "0 8 * * *", "ping"); throw new Error("expected add to reject"); } catch (error) { assert(error instanceof PluginCronControlClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_CRON_CONTROL", message: "nested plugin cron control failure" } }); assertEqual(error.message, "nested plugin cron control failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
