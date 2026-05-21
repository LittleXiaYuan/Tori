import { createDispatchWorkersClient, DispatchWorkersClientError } from "./dispatch-workers";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DispatchWorkersClient lists workers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchWorkersClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ workers: [{ id: "w1", type: "cursor", capabilities: ["coding"] }], count: 1 }); } });
  const result = await client.list();
  assertEqual(result.count, 1);
  assertEqual(result.workers[0]?.id, "w1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("DispatchWorkersClient reads worker detail with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchWorkersClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ id: "w 1/二", type: "claude_code", capabilities: ["docs"] }); } });
  const result = await client.detail("w 1/二");
  assertEqual(result.type, "claude_code");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers/detail?id=w+1%2F%E4%BA%8C");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("DispatchWorkersClient exposes dispatch-workers nested gateway errors", async () => {
  const client = createDispatchWorkersClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DISPATCH_WORKERS", message: "nested dispatch workers failure" } }, { status: 404 }) });
  try { await client.detail("missing"); throw new Error("expected detail to reject"); } catch (error) { assert(error instanceof DispatchWorkersClientError); assertEqual(error.name, "DispatchClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { code: "DISPATCH_WORKERS", message: "nested dispatch workers failure" } }); assertEqual(error.message, "nested dispatch workers failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
