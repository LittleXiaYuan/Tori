import { createSetupProviderClient, SetupProviderClientError } from "./setup-provider";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SetupProviderClient tests providers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupProviderClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, provider: { base_url: "https://api.deepseek.com/v1", available: true } }); } });
  const result = await client.test({ base_url: "https://api.deepseek.com/v1", api_key: "sk-test", model: "deepseek-chat" });
  assertEqual(result.ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/test-provider");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { base_url: "https://api.deepseek.com/v1", api_key: "sk-test", model: "deepseek-chat" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SetupProviderClient applies setup templates with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupProviderClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, status: "applied", applied: "hybrid", persisted: true, restart_required: true }); } });
  const result = await client.apply({ template_id: "hybrid", base_url: "https://api.deepseek.com/v1", api_key: "sk-test", model: "deepseek-chat", overrides: { sandbox_tier: "local" } });
  assertEqual(result.restart_required, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/apply");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { template_id: "hybrid", base_url: "https://api.deepseek.com/v1", api_key: "sk-test", model: "deepseek-chat", overrides: { sandbox_tier: "local" } });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SetupProviderClient exposes nested setup provider errors", async () => {
  const client = createSetupProviderClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested setup provider failure" } }, { status: 400 }) });
  try { await client.apply({ template_id: "" }); throw new Error("expected apply to reject"); } catch (error) { assert(error instanceof SetupProviderClientError); assertEqual(error.name, "SetupClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested setup provider failure" } }); assertEqual(error.message, "nested setup provider failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
