import { createPluginExtensionRegisterClient, PluginExtensionRegisterClientError } from "./plugin-extension-register";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginExtensionRegisterClient registers all extension kinds with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginExtensionRegisterClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); const value = String(url); if (value.includes("provider")) return jsonResponse({ ok: true, provider_id: "p1" }); if (value.includes("channel")) return jsonResponse({ ok: true, channel: "c1" }); if (value.includes("search")) return jsonResponse({ ok: true, search: "s1" }); if (value.includes("guardrail")) return jsonResponse({ ok: true, guardrail: "g1" }); if (value.includes("embedding")) return jsonResponse({ ok: true, embedding: "e1" }); return jsonResponse({ ok: true, speech: "tts" }); } });
  assertEqual((await client.provider({ id: "p1" })).provider_id, "p1");
  assertEqual((await client.channel({ name: "c1" })).channel, "c1");
  assertEqual((await client.search({ name: "s1" })).search, "s1");
  assertEqual((await client.guardrail({ name: "g1" })).guardrail, "g1");
  assertEqual((await client.embedding({ name: "e1" })).embedding, "e1");
  assertEqual((await client.speech({ name: "tts" })).speech, "tts");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/register/provider");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/plugin-api/register/channel");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/plugin-api/register/search");
  assertEqual(calls[3]?.url, "http://localhost:9090/v1/plugin-api/register/guardrail");
  assertEqual(calls[4]?.url, "http://localhost:9090/v1/plugin-api/register/embedding");
  assertEqual(calls[5]?.url, "http://localhost:9090/v1/plugin-api/register/speech");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "p1" });
});

test("PluginExtensionRegisterClient exposes plugin-extension-register nested gateway errors", async () => {
  const client = createPluginExtensionRegisterClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_EXTENSION_REGISTER", message: "nested plugin extension register failure" } }, { status: 403 }) });
  try { await client.provider({ id: "p1" }); throw new Error("expected provider to reject"); } catch (error) { assert(error instanceof PluginExtensionRegisterClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_EXTENSION_REGISTER", message: "nested plugin extension register failure" } }); assertEqual(error.message, "nested plugin extension register failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
