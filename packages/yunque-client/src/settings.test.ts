import { createSettingsClient, SettingsClientError } from "./settings";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SettingsClient reads schema and config with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/schema")) return jsonResponse({ groups: [{ id: "llm", fields: [{ key: "LLM_BASE_URL" }] }] }); return jsonResponse({ values: { LLM_BASE_URL: "http://localhost:11434", LLM_API_KEY: "******" } }); } });
  const schema = await client.schema(); const config = await client.config();
  assertEqual(schema.groups[0]?.id, "llm"); assertEqual(config.values.LLM_API_KEY, "******");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/settings/schema"); assertEqual(calls[1]?.url, "http://localhost:9090/api/settings/config"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SettingsClient updates config and reloads runtime providers with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "PUT") return jsonResponse({ success: true, restart_required: true, message: "saved" }); return jsonResponse({ success: true, reloaded: ["smart"], message: "ok" }); } });
  const saved = await client.updateConfig({ LLM_BASE_URL: "http://llm", LLM_MODEL: "qwen" }); const reloaded = await client.reload();
  assertEqual(saved.restart_required, true); assertEqual(reloaded.reloaded?.[0], "smart");
  assertEqual(calls[0]?.init?.method, "PUT"); assertEqual(calls[0]?.init?.body, JSON.stringify({ values: { LLM_BASE_URL: "http://llm", LLM_MODEL: "qwen" } })); assertEqual(calls[1]?.url, "http://localhost:9090/v1/config/reload"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("SettingsClient checks setup and detects host directories", async () => {
  const calls: string[] = [];
  const client = createSettingsClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); if (String(url).endsWith("/check")) return jsonResponse({ env_exists: true, has_llm_key: true, api_ok: false, setup_needed: false }); return jsonResponse({ dirs: { documents: "C:/Users/A/Documents" }, default_paths: ["C:/Users/A/Documents"], current_read: "C:/Code", current_write: "C:/Code/out" }); } });
  const check = await client.check(); const dirs = await client.detectDirs();
  assertEqual(check.setup_needed, false); assertEqual(dirs.default_paths?.[0], "C:/Users/A/Documents"); assertEqual(calls[0], "http://localhost:9090/api/settings/check"); assertEqual(calls[1], "http://localhost:9090/api/settings/detect-dirs");
});

test("SettingsClient manages backup info export and import", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/info")) return jsonResponse({ files: { "config/.env": 12 }, file_count: 1, total_bytes: 12, version: "dev" }); if (path.endsWith("/export")) return new Response(new Blob(["zipdata"]), { status: 200, headers: { "Content-Type": "application/zip", "Content-Disposition": 'attachment; filename="yunque-backup-demo.zip"' } }); return jsonResponse({ success: true, restored: 2, skipped: 0 }); } });
  const info = await client.backupInfo(); const exported = await client.exportBackup(); const imported = await client.importBackup(new Blob(["zipdata"], { type: "application/zip" }), "restore.zip");
  assertEqual(info.file_count, 1); assertEqual(exported.filename, "yunque-backup-demo.zip"); assertEqual(exported.contentType, "application/zip"); assertEqual(imported.restored, 2);
  assert(calls[2]?.init?.body instanceof FormData); assertEqual(new Headers(calls[2]?.init?.headers).get("content-type"), null, "multipart boundary must be set by fetch");
});

test("SettingsClient throws SettingsClientError with parsed and text bodies", async () => {
  const jsonClient = createSettingsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "invalid api key" }, { status: 401 }) });
  try { await jsonClient.config(); throw new Error("expected config to reject"); } catch (error) { assert(error instanceof SettingsClientError); assertEqual(error.status, 401); assertDeepEqual(error.body, { error: "invalid api key" }); assertEqual(error.message, "invalid api key"); }
  const nestedClient = createSettingsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "settings key is required" } }, { status: 400 }) });
  try { await nestedClient.updateConfig({}); throw new Error("expected updateConfig to reject"); } catch (error) { assert(error instanceof SettingsClientError); assertEqual(error.status, 400); assertEqual(error.message, "settings key is required"); }
  const textClient = createSettingsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET only", { status: 405 }) });
  try { await textClient.schema(); throw new Error("expected schema to reject"); } catch (error) { assert(error instanceof SettingsClientError); assertEqual(error.status, 405); assertEqual(error.body, "GET only"); assertEqual(error.message, "GET only"); }
});
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
