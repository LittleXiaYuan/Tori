import { createAdminClient, AdminClientError } from "./admin";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AdminClient controls desktop console and autostart with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAdminClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("console")) return jsonResponse({ console_hidden: init?.method === "POST" }); return jsonResponse({ autostart_enabled: init?.method === "POST" }); } });
  const consoleStatus = await client.consoleStatus(); const toggledConsole = await client.toggleConsole(); const autostartStatus = await client.autostartStatus(); const toggledAutostart = await client.toggleAutostart();
  assertEqual(consoleStatus.console_hidden, false); assertEqual(toggledConsole.console_hidden, true); assertEqual(autostartStatus.autostart_enabled, false); assertEqual(toggledAutostart.autostart_enabled, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/desktop/console"); assertEqual(calls[1]?.init?.method, "POST"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("AdminClient lists and creates tenants with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAdminClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "t2", name: "team", api_key: "ya_test" }, { status: 201 }); return jsonResponse({ tenants: [{ id: "t1", name: "default" }], count: 1 }); } });
  const list = await client.listTenants(); const created = await client.createTenant("team");
  assertEqual(list.count, 1); assertEqual(created.name, "team"); assertEqual(created.api_key, "ya_test"); assertEqual(calls[1]?.init?.body, JSON.stringify({ name: "team" })); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("AdminClient translates and executes natural-language config", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAdminClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/translate")) return jsonResponse({ status: "ok", result: { intent: "model_switch" }, executed: false }); return jsonResponse({ status: "ok", result: { intent: "model_switch", executed_at: "now" }, executed: true }); } });
  const parsed = await client.nlConfigTranslate("切换到 qwen"); const executed = await client.nlConfig({ text: "切换到 qwen", execute: true });
  assertEqual(parsed.executed, false); assertEqual(executed.executed, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/nl-config/translate"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/nl-config");
});

test("AdminClient returns partial NL config 422 bodies as results", async () => {
  const client = createAdminClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ status: "partial", result: { intent: "kb_add", exec_error: "source missing" }, executed: true }, { status: 422 }) });
  const result = await client.nlConfig({ text: "添加知识库", execute: true });
  assertEqual(result.status, "partial"); assertEqual(result.executed, true);
});

test("AdminClient throws AdminClientError with parsed and text bodies", async () => {
  const jsonClient = createAdminClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "invalid api key" }, { status: 401 }) });
  try { await jsonClient.listTenants(); throw new Error("expected listTenants to reject"); } catch (error) { assert(error instanceof AdminClientError); assertEqual(error.status, 401); assertDeepEqual(error.body, { error: "invalid api key" }); assertEqual(error.message, "invalid api key"); }
  const nestedClient = createAdminClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "tenant name is required" } }, { status: 400 }) });
  try { await nestedClient.createTenant(""); throw new Error("expected createTenant to reject"); } catch (error) { assert(error instanceof AdminClientError); assertEqual(error.status, 400); assertEqual(error.message, "tenant name is required"); }
  const textClient = createAdminClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST only", { status: 405 }) });
  try { await textClient.createTenant("x"); throw new Error("expected createTenant to reject"); } catch (error) { assert(error instanceof AdminClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST only"); assertEqual(error.message, "POST only"); }
});
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
