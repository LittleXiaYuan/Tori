import { createDispatchReadClient, DispatchReadClientError } from "./dispatch-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DispatchReadClient lists and reads workers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("detail")) return jsonResponse({ id: "w1", type: "claude_code", capabilities: ["docs"] }); return jsonResponse({ workers: [{ id: "w1", type: "cursor", capabilities: ["coding"] }], count: 1 }); } });
  assertEqual((await client.workers()).count, 1);
  assertEqual((await client.worker("w1")).type, "claude_code");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/workers/detail?id=w1");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("DispatchReadClient reads queue and worker config with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchReadClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("queue")) return jsonResponse({ message: "dispatch queue" }); return jsonResponse({ type: "cursor", mcp_config: "{}", instructions: "Register worker", server_url: "http://localhost:9090/mcp/v1" }); } });
  assertEqual((await client.queue()).message, "dispatch queue");
  assertEqual((await client.workerConfig("cursor")).type, "cursor");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/dispatch/queue");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/workers/config?type=cursor");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("DispatchReadClient exposes nested read errors", async () => {
  const client = createDispatchReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DISPATCH_READ", message: "worker not found" } }, { status: 404 }) });
  try { await client.worker("missing"); throw new Error("expected worker to reject"); } catch (error) { assert(error instanceof DispatchReadClientError); assertEqual(error.name, "DispatchClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { code: "DISPATCH_READ", message: "worker not found" } }); assertEqual(error.message, "worker not found"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
