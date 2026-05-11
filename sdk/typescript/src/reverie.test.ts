import { createReverieClient, ReverieClientError } from "./reverie";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ReverieClient reads journal with bearer token and filters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ thoughts: [{ id: "t1", category: "task" }], total: 1, limit: 10, offset: 0 }); } });
  const result = await client.journal({ category: "task", min_significance: 0.5, delivered: false, limit: 10, offset: 0 });
  assertEqual(result.total, 1); assertEqual(result.thoughts?.[0]?.id, "t1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/reverie/journal?category=task&min_significance=0.5&delivered=false&limit=10&offset=0");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ReverieClient reads stats and config with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/stats")) return jsonResponse({ total: 3 }); return jsonResponse({ config: { enabled: true }, running: true }); } });
  const stats = await client.stats() as { total?: number }; const config = await client.config();
  assertEqual(stats.total, 3); assertEqual(config.running, true); assertEqual((config.config as { enabled?: boolean }).enabled, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/reverie/stats"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/reverie/config");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ReverieClient updates config and triggers think", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "PUT") return jsonResponse({ config: { enabled: false }, running: false }); return jsonResponse({ thought: { id: "t2", category: "event" } }); } });
  const updated = await client.updateConfig({ enabled: false, interval_minutes: 20, min_significance: 0.7 }); const thought = await client.think({ event_type: "task_completed", trigger: "demo" });
  assertEqual(updated.running, false); assertEqual(thought.thought.id, "t2");
  assertEqual(calls[0]?.init?.method, "PUT"); assertEqual(calls[0]?.init?.body, JSON.stringify({ enabled: false, interval_minutes: 20, min_significance: 0.7 }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/reverie/think");
});

test("ReverieClient deletes thoughts and reads actions and targets", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReverieClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/thought")) return jsonResponse({ deleted: true, id: "t1" }); if (String(url).includes("/actions")) return jsonResponse({ actions: [{ id: "a1" }], total: 1 }); return jsonResponse({ targets: [{ channel: "feishu", targets: ["u1"], env_var: "REVERIE_TARGET_FEISHU" }], count: 1, env_prefix: "REVERIE_TARGET_" }); } });
  const deleted = await client.deleteThought("t1"); const actions = await client.actions(); const targets = await client.targets();
  assertEqual(deleted.deleted, true); assertEqual(actions.total, 1); assertEqual(targets.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/reverie/thought?id=t1"); assertEqual(calls[0]?.init?.method, "DELETE");
});

test("ReverieClient throws ReverieClientError with parsed and text bodies", async () => {
  const jsonClient = createReverieClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "reverie not initialized" }, { status: 404 }) });
  try { await jsonClient.stats(); throw new Error("expected stats to reject"); } catch (error) { assert(error instanceof ReverieClientError); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: "reverie not initialized" }); assertEqual(error.message, "reverie not initialized"); }
  const textClient = createReverieClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST only", { status: 405 }) });
  try { await textClient.think(); throw new Error("expected think to reject"); } catch (error) { assert(error instanceof ReverieClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST only"); assertEqual(error.message, "POST only"); }
});
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
