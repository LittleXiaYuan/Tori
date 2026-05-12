import { createPluginCronReadClient, PluginCronReadClientError } from "./plugin-cron-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginCronReadClient lists filtered plugin cron jobs with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginCronReadClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ jobs: [{ id: "cron-1" }] }); } });
  assertEqual((await client.list("demo/plugin")).jobs.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/cron/list?plugin=demo%2Fplugin");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
});

test("PluginCronReadClient lists all plugin cron jobs without plugin filter", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginCronReadClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ jobs: [] }); } });
  await client.list();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/cron/list");
});

test("PluginCronReadClient exposes plugin-cron-read nested gateway errors", async () => {
  const client = createPluginCronReadClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_CRON_READ", message: "nested plugin cron read failure" } }, { status: 403 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof PluginCronReadClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_CRON_READ", message: "nested plugin cron read failure" } }); assertEqual(error.message, "nested plugin cron read failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
