import { createSettingsSchemaClient, SettingsSchemaClientError } from "./settings-schema";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SettingsSchemaClient reads schema with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsSchemaClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ groups: [{ id: "llm", fields: [{ key: "LLM_MODEL" }] }] }); } });
  const schema = await client.schema();
  assertEqual(schema.groups[0]?.id, "llm"); assertEqual(calls[0]?.url, "http://localhost:9090/api/settings/schema"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SettingsSchemaClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSettingsSchemaClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ groups: [] }); } });
  await client.schema();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SettingsSchemaClient exposes nested schema errors", async () => {
  const client = createSettingsSchemaClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SETTINGS_SCHEMA", message: "nested schema failed" } }, { status: 500 }) });
  try { await client.schema(); throw new Error("expected schema to reject"); } catch (error) { assert(error instanceof SettingsSchemaClientError); assertEqual(error.name, "SettingsClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "SETTINGS_SCHEMA", message: "nested schema failed" } }); assertEqual(error.message, "nested schema failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
