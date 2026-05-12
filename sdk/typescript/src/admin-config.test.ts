import { createAdminConfigClient, AdminConfigClientError } from "./admin-config";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AdminConfigClient translates and executes natural-language config with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAdminConfigClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/translate")) return jsonResponse({ status: "ok", result: { intent: "model_switch" }, executed: false }); return jsonResponse({ status: "ok", result: { intent: "model_switch", executed_at: "now" }, executed: true }); } });
  assertEqual((await client.nlConfigTranslate("切换到 qwen")).executed, false);
  assertEqual((await client.nlConfig({ text: "切换到 qwen", execute: true })).executed, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/nl-config/translate");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/nl-config");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("AdminConfigClient returns partial 422 bodies as results", async () => {
  const client = createAdminConfigClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ status: "partial", result: { intent: "kb_add", exec_error: "source missing" }, executed: true }, { status: 422 }) });
  const result = await client.nlConfig({ text: "添加知识库", execute: true });
  assertEqual(result.status, "partial");
  assertEqual(result.executed, true);
});

test("AdminConfigClient exposes nested config errors", async () => {
  const client = createAdminConfigClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "CONFIG", message: "text required" } }, { status: 400 }) });
  try { await client.nlConfigTranslate(""); throw new Error("expected translate to reject"); } catch (error) { assert(error instanceof AdminConfigClientError); assertEqual(error.name, "AdminClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "CONFIG", message: "text required" } }); assertEqual(error.message, "text required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
