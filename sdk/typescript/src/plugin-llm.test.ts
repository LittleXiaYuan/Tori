import { createPluginLLMClient, PluginLLMClientError } from "./plugin-llm";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginLLMClient calls plugin LLM with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginLLMClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ reply: "ok" }); } });
  const result = await client.complete({ messages: [{ role: "user", content: "hi" }], temperature: 0.2, model: "demo" });
  assertEqual(result.reply, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/llm");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { messages: [{ role: "user", content: "hi" }], temperature: 0.2, model: "demo" });
});

test("PluginLLMClient preserves custom headers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginLLMClient({ baseUrl: "http://localhost:9090", token: "plg_demo", headers: { "X-Plugin-Run": "run-1" }, fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ reply: "ok" }); } });
  await client.complete({ messages: [{ role: "system", content: "brief" }] });
  const headers = new Headers(calls[0]?.init?.headers);
  assertEqual(headers.get("x-plugin-run"), "run-1");
  assertEqual(headers.get("content-type"), "application/json");
});

test("PluginLLMClient exposes nested plugin LLM errors", async () => {
  const client = createPluginLLMClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "PLUGIN_SCOPE", message: "plugin llm scope is required" } }, { status: 403 }) });
  try { await client.complete({ messages: [] }); throw new Error("expected complete to reject"); } catch (error) { assert(error instanceof PluginLLMClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PLUGIN_SCOPE", message: "plugin llm scope is required" } }); assertEqual(error.message, "plugin llm scope is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
