import { createPluginControlClient, PluginControlClientError } from "./plugin-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginControlClient toggles plugins with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ name: "demo", enabled: false, skills_count: 3 }); } });
  const result = await client.toggle("demo", false);
  assertEqual(result.name, "demo");
  assertEqual(result.enabled, false);
  assertEqual(result.skills_count, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/toggle");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "demo", enabled: false });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PluginControlClient reads ui reloads and opens folders with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginControlClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("ui")) return jsonResponse({ tabs: [{ id: "demo" }] }); if (String(url).includes("reload")) return jsonResponse({ status: "reloaded", skills: 5 }); return jsonResponse({ ok: true, path: "C:/plugins/demo" }); } });
  const ui = await client.ui();
  const reload = await client.reload();
  const opened = await client.openFolder("demo");
  assertEqual(ui.tabs.length, 1);
  assertEqual(reload.skills, 5);
  assertEqual(opened.path, "C:/plugins/demo");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/ui");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/plugins/reload");
  assertEqual(calls[1]?.init?.method, "POST");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/plugins/open-folder?name=demo");
  assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "ya");
});

test("PluginControlClient supports opening the root plugin folder", async () => {
  const calls: string[] = [];
  const client = createPluginControlClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); return jsonResponse({ ok: true, path: "C:/plugins" }); } });
  const result = await client.openFolder();
  assertEqual(result.ok, true);
  assertEqual(calls[0], "http://localhost:9090/v1/plugins/open-folder");
});

test("PluginControlClient exposes control nested gateway errors", async () => {
  const client = createPluginControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested plugin control failure" } }, { status: 400 }) });
  try { await client.toggle("", true); throw new Error("expected toggle to reject"); } catch (error) { assert(error instanceof PluginControlClientError); assertEqual(error.name, "PluginsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested plugin control failure" } }); assertEqual(error.message, "nested plugin control failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
