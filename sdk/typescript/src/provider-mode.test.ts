import { createProviderModeClient, ProviderModeClientError } from "./provider-mode";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ProviderModeClient reads and sets mode with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderModeClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ mode: "hybrid", ok: true }); return jsonResponse({ mode: "local", bound: true }); } });
  const current = await client.getMode(); const updated = await client.setMode("hybrid");
  assertEqual(current.mode, "local"); assertEqual(updated.mode, "hybrid"); assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/mode"); assertEqual(calls[1]?.url, "http://localhost:9090/api/providers/mode"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { mode: "hybrid" });
});

test("ProviderModeClient reads and sets exec provider with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderModeClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ exec_provider: "deepseek", ok: true }); return jsonResponse({ exec_provider: "local", available_providers: ["local", "deepseek"] }); } });
  const current = await client.getExecProvider(); const updated = await client.setExecProvider("deepseek");
  assertEqual(current.exec_provider, "local"); assertEqual(updated.exec_provider, "deepseek"); assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/exec"); assertEqual(calls[1]?.url, "http://localhost:9090/api/providers/exec"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { provider_id: "deepseek" });
});

test("ProviderModeClient exposes nested mode errors", async () => {
  const client = createProviderModeClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "mode is required" } }, { status: 400 }) });
  try { await client.setMode(""); throw new Error("expected setMode to reject"); } catch (error) { assert(error instanceof ProviderModeClientError); assertEqual(error.name, "ProvidersClientError"); assertEqual(error.status, 400); assertEqual(error.message, "mode is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
