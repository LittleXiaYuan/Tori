import { createPluginApiClient, PluginApiClientError } from "./plugin-api";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PluginApiClient calls LLM search and send with plugin bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginApiClient({ baseUrl: "http://localhost:9090/", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/llm")) return jsonResponse({ reply: "ok" }); if (String(url).includes("/search")) return jsonResponse({ results: [{ title: "doc" }] }); return jsonResponse({ ok: true }); } });
  const llm = await client.llm({ messages: [{ role: "user", content: "hi" }], temperature: 0.2 }); const search = await client.search("agent", 3); const sent = await client.send("webhook", "ops", "hello", "text");
  assertEqual(llm.reply, "ok"); assertEqual(search.results.length, 1); assertEqual(sent.ok, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/plugin-api/llm"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer plg_demo"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { query: "agent", limit: 3 });
});

test("PluginApiClient manages plugin memory and agent memory", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginApiClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); const value = String(url); if (value.includes("/memory/get")) return jsonResponse({ value: "bar" }); if (value.includes("/memory/list")) return jsonResponse({ entries: [{ key: "foo" }] }); if (value.includes("/memory/search")) return jsonResponse({ results: [{ key: "foo" }] }); if (value.includes("agent-memory/search")) return jsonResponse({ context: "remembered" }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.memoryGet("foo")).value, "bar"); assertEqual((await client.memorySet("foo", "bar")).ok, true); assertEqual((await client.memoryDelete("foo")).ok, true); assertEqual((await client.memoryList("f")).entries.length, 1); assertEqual((await client.memorySearch("foo", 2)).results.length, 1); assertEqual((await client.agentMemorySearch("foo", 5)).context, "remembered"); assertEqual((await client.agentMemoryAdd("fact")).ok, true); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { key: "foo" });
});

test("PluginApiClient manages knowledge and plugin cron", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginApiClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); const value = String(url); if (value.includes("knowledge/search")) return jsonResponse({ results: [{ id: "k1" }] }); if (value.includes("cron/add")) return jsonResponse({ id: "cron-1", status: "created" }); if (value.includes("cron/list")) return jsonResponse({ jobs: [{ id: "cron-1" }] }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.knowledgeSearch("sdk", 4)).results.length, 1); assertEqual((await client.knowledgeIngest("content", "plugin", "note.md")).ok, true); assertEqual((await client.cronAdd("daily", "0 8 * * *", "ping")).id, "cron-1"); assertEqual((await client.cronRemove("cron-1")).ok, true); assertEqual((await client.cronList("demo")).jobs.length, 1); assertEqual(calls[4]?.url, "http://localhost:9090/v1/plugin-api/cron/list?plugin=demo");
});

test("PluginApiClient registers extensions and lists them", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPluginApiClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("extensions")) return jsonResponse({ extensions: [{ type: "provider" }] }); if (String(url).includes("provider")) return jsonResponse({ ok: true, provider_id: "p1" }); if (String(url).includes("channel")) return jsonResponse({ ok: true, channel: "c1" }); if (String(url).includes("search")) return jsonResponse({ ok: true, search: "s1" }); if (String(url).includes("guardrail")) return jsonResponse({ ok: true, guardrail: "g1" }); if (String(url).includes("embedding")) return jsonResponse({ ok: true, embedding: "e1" }); return jsonResponse({ ok: true, speech: "tts" }); } });
  assertEqual((await client.registerProvider({ id: "p1" })).provider_id, "p1"); assertEqual((await client.registerChannel({ name: "c1" })).channel, "c1"); assertEqual((await client.registerSearch({ name: "s1" })).search, "s1"); assertEqual((await client.registerGuardrail({ name: "g1" })).guardrail, "g1"); assertEqual((await client.registerEmbedding({ name: "e1" })).embedding, "e1"); assertEqual((await client.registerSpeech({ name: "tts" })).speech, "tts"); assertEqual((await client.extensions()).extensions.length, 1); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "p1" });
});

test("PluginApiClient throws PluginApiClientError with parsed and text bodies", async () => {
  const jsonClient = createPluginApiClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: "permission denied" }, { status: 403 }) });
  try { await jsonClient.llm({ messages: [] }); throw new Error("expected llm to reject"); } catch (error) { assert(error instanceof PluginApiClientError); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: "permission denied" }); assertEqual(error.message, "permission denied"); }
  const nestedClient = createPluginApiClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "plugin permission scope is required" } }, { status: 400 }) });
  try { await nestedClient.llm({ messages: [] }); throw new Error("expected llm to reject"); } catch (error) { assert(error instanceof PluginApiClientError); assertEqual(error.status, 400); assertEqual(error.message, "plugin permission scope is required"); }
  const textClient = createPluginApiClient({ baseUrl: "http://localhost:9090", token: "plg_demo", fetch: async () => new Response("missing plugin token", { status: 401 }) });
  try { await textClient.extensions(); throw new Error("expected extensions to reject"); } catch (error) { assert(error instanceof PluginApiClientError); assertEqual(error.status, 401); assertEqual(error.body, "missing plugin token"); assertEqual(error.message, "missing plugin token"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
