import { createSettingsConfigClient, SettingsConfigClientError } from "./settings-config";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SettingsConfigClient reads schema and config with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsConfigClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/schema")) return jsonResponse({ groups: [{ id: "llm" }] }); return jsonResponse({ values: { LLM_API_KEY: "******" } }); } });
  assertEqual((await client.schema()).groups[0]?.id, "llm");
  assertEqual((await client.config()).values.LLM_API_KEY, "******");
  assertDeepEqual(calls.map((call) => call.url), ["http://localhost:9090/api/settings/schema", "http://localhost:9090/api/settings/config"]);
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SettingsConfigClient updates config and reloads with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsConfigClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "PUT") return jsonResponse({ success: true, restart_required: true }); return jsonResponse({ success: true, reloaded: ["smart"] }); } });
  assertEqual((await client.updateConfig({ LLM_MODEL: "qwen" })).restart_required, true);
  assertEqual((await client.reload()).reloaded?.[0], "smart");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/settings/config");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { values: { LLM_MODEL: "qwen" } });
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/config/reload");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SettingsConfigClient checks setup and detects dirs", async () => {
  const calls: string[] = [];
  const client = createSettingsConfigClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); if (String(url).endsWith("/check")) return jsonResponse({ setup_needed: false, api_ok: true }); return jsonResponse({ dirs: { documents: "C:/Users/A/Documents" }, default_paths: ["C:/Users/A/Documents"] }); } });
  assertEqual((await client.check()).setup_needed, false);
  assertEqual((await client.detectDirs()).default_paths?.[0], "C:/Users/A/Documents");
  assertDeepEqual(calls, ["http://localhost:9090/api/settings/check", "http://localhost:9090/api/settings/detect-dirs"]);
});

test("SettingsConfigClient exposes nested config errors", async () => {
  const client = createSettingsConfigClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SETTINGS_CONFIG", message: "config failed" } }, { status: 400 }) });
  try { await client.updateConfig({}); throw new Error("expected updateConfig to reject"); } catch (error) { assert(error instanceof SettingsConfigClientError); assertEqual(error.name, "SettingsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "SETTINGS_CONFIG", message: "config failed" } }); assertEqual(error.message, "config failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
