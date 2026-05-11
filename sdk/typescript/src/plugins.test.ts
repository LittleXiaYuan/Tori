import { createPluginsClient, PluginsClientError } from "./plugins";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginsClient lists and toggles plugins with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/toggle")) return jsonResponse({ name: "demo", enabled: false, skills_count: 3 }); return jsonResponse({ plugins: [{ name: "demo", enabled: true }] }); } });
  const list = await client.list(); const toggled = await client.toggle("demo", false);
  assertEqual(list.plugins[0]?.name, "demo"); assertEqual(toggled.skills_count, 3); assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/plugins/toggle"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { name: "demo", enabled: false });
});

test("PluginsClient creates and deletes plugins with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("delete")) return jsonResponse({ status: "deleted", name: "demo" }); return jsonResponse({ status: "created", name: "demo", dir: "demo", full_path: "C:/tmp/demo" }, { status: 201 }); } });
  const created = await client.create({ name: "demo", language: "python", template: "basic", skills: [{ name: "hello" }] }); const deleted = await client.delete("demo");
  assertEqual(created.status, "created"); assertEqual(deleted.status, "deleted"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/create"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/plugins/delete?name=demo"); assertEqual(calls[1]?.init?.method, "DELETE"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("PluginsClient reads and saves plugin files", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "PUT") return jsonResponse({ status: "saved" }); return jsonResponse({ files: [{ name: "plugin.json", content: "{}", size: 2 }] }); } });
  const files = await client.files("demo"); const saved = await client.saveFile("demo", "plugin.json", "{\"name\":\"demo\"}");
  assertEqual(files.files[0]?.name, "plugin.json"); assertEqual(saved.status, "saved"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugins/files?name=demo"); assertEqual(calls[1]?.init?.method, "PUT"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { file: "plugin.json", content: "{\"name\":\"demo\"}" });
});

test("PluginsClient reads ui tabs reloads and opens plugin folder", async () => {
  const calls: string[] = [];
  const client = createPluginsClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); if (String(url).includes("ui")) return jsonResponse({ tabs: [{ id: "demo" }] }); if (String(url).includes("reload")) return jsonResponse({ status: "reloaded", skills: 5 }); return jsonResponse({ ok: true, path: "C:/plugins/demo" }); } });
  const ui = await client.ui(); const reload = await client.reload(); const opened = await client.openFolder("demo");
  assertEqual(ui.tabs.length, 1); assertEqual(reload.skills, 5); assertEqual(opened.ok, true); assertEqual(calls[2], "http://localhost:9090/v1/plugins/open-folder?name=demo");
});

test("PluginsClient throws PluginsClientError with parsed and text bodies", async () => {
  const jsonClient = createPluginsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "plugin not found" }, { status: 404 }) });
  try { await jsonClient.delete("missing"); throw new Error("expected delete to reject"); } catch (error) { assert(error instanceof PluginsClientError); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: "plugin not found" }); assertEqual(error.message, "plugin not found"); }
  const textClient = createPluginsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST only", { status: 405 }) });
  try { await textClient.reload(); throw new Error("expected reload to reject"); } catch (error) { assert(error instanceof PluginsClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST only"); assertEqual(error.message, "POST only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
