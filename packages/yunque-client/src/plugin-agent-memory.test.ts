import { createPluginAgentMemoryClient, PluginAgentMemoryClientError } from "./plugin-agent-memory";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginAgentMemoryClient searches and adds agent memory with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginAgentMemoryClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/search")) return jsonResponse({ context: "remembered" }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.search("foo", 5)).context, "remembered");
  assertEqual((await client.add("fact", "plugin-test")).ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/agent-memory/search");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { query: "foo", top_k: 5 });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { fact: "fact", source: "plugin-test" });
});

test("PluginAgentMemoryClient allows omitted topK and source", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginAgentMemoryClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/search")) return jsonResponse({ context: "" }); return jsonResponse({ ok: true }); } });
  await client.search("foo");
  await client.add("fact");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { query: "foo" });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { fact: "fact" });
});

test("PluginAgentMemoryClient exposes nested agent memory errors", async () => {
  const client = createPluginAgentMemoryClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "AGENT_MEMORY", message: "agent memory scope is required" } }, { status: 403 }) });
  try { await client.search("foo"); throw new Error("expected search to reject"); } catch (error) { assert(error instanceof PluginAgentMemoryClientError); assertEqual(error.name, "PluginApiClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "AGENT_MEMORY", message: "agent memory scope is required" } }); assertEqual(error.message, "agent memory scope is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
