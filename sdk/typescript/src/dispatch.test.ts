import { createDispatchClient, DispatchClientError } from "./dispatch";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DispatchClient lists workers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ workers: [{ id: "w1", type: "cursor", capabilities: ["coding"] }], count: 1 }); } });
  const result = await client.workers();
  assertEqual(result.workers[0]?.id, "w1"); assertEqual(result.count, 1); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("DispatchClient reads and removes workers with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/remove")) return jsonResponse({ status: "removed" }); return jsonResponse({ id: "w1", type: "claude_code", capabilities: ["docs"] }); } });
  const worker = await client.worker("w1"); const removed = await client.removeWorker("w1");
  assertEqual(worker.type, "claude_code"); assertEqual(removed.status, "removed"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers/detail?id=w1"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/workers/remove"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "w1" });
});

test("DispatchClient reads queue and enqueues tasks", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ task_id: "t1", status: "enqueued" }); return jsonResponse({ message: "dispatch queue (use task system for now)" }); } });
  const queue = await client.queue(); const enqueued = await client.enqueue({ task_id: "t1", capabilities: ["coding", "testing"], priority: 10 });
  assertEqual(queue.message, "dispatch queue (use task system for now)"); assertEqual(enqueued.status, "enqueued"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/dispatch/queue"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/dispatch/enqueue"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { task_id: "t1", capabilities: ["coding", "testing"], priority: 10 });
});

test("DispatchClient reads worker config with type query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ type: "cursor", mcp_config: "{\"mcpServers\":{}}", instructions: "Register worker", server_url: "http://localhost:9090/mcp/v1" }); } });
  const config = await client.workerConfig("cursor");
  assertEqual(config.type, "cursor"); assertEqual(config.server_url, "http://localhost:9090/mcp/v1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers/config?type=cursor");
});

test("DispatchClient throws DispatchClientError with parsed and text bodies", async () => {
  const jsonClient = createDispatchClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "worker not found" }, { status: 404 }) });
  try { await jsonClient.worker("missing"); throw new Error("expected worker to reject"); } catch (error) { assert(error instanceof DispatchClientError); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: "worker not found" }); assertEqual(error.message, "worker not found"); }
  const textClient = createDispatchClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.workers(); throw new Error("expected workers to reject"); } catch (error) { assert(error instanceof DispatchClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
