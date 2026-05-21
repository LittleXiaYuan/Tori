import { createSettingsRuntimeClient, SettingsRuntimeClientError } from "./settings-runtime";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SettingsRuntimeClient checks setup and reloads with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsRuntimeClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/check")) return jsonResponse({ setup_needed: false, api_ok: true }); return jsonResponse({ success: true, reloaded: ["smart"] }); } });
  assertEqual((await client.check()).api_ok, true); assertEqual((await client.reload()).reloaded?.[0], "smart");
  assertDeepEqual(calls.map((call) => call.url), ["http://localhost:9090/api/settings/check", "http://localhost:9090/v1/config/reload"]); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), {});
});

test("SettingsRuntimeClient detects dirs with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsRuntimeClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ dirs: { documents: "C:/Users/A/Documents" }, default_paths: ["C:/Users/A/Documents"] }); } });
  assertEqual((await client.detectDirs()).default_paths?.[0], "C:/Users/A/Documents"); assertEqual(calls[0]?.url, "http://localhost:9090/api/settings/detect-dirs"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SettingsRuntimeClient exposes nested runtime errors", async () => {
  const client = createSettingsRuntimeClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SETTINGS_RUNTIME", message: "nested runtime failed" } }, { status: 503 }) });
  try { await client.reload(); throw new Error("expected reload to reject"); } catch (error) { assert(error instanceof SettingsRuntimeClientError); assertEqual(error.name, "SettingsClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "SETTINGS_RUNTIME", message: "nested runtime failed" } }); assertEqual(error.message, "nested runtime failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
