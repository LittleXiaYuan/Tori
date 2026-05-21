import { createDispatchWorkerConfigClient, DispatchWorkerConfigClientError } from "./dispatch-worker-config";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DispatchWorkerConfigClient reads typed config with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchWorkerConfigClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ type: "cursor", mcp_config: "{}", instructions: "Register worker", server_url: "http://localhost:9090/mcp/v1" }); } });
  const result = await client.get("cursor");
  assertEqual(result.type, "cursor");
  assertEqual(result.server_url, "http://localhost:9090/mcp/v1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers/config?type=cursor");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("DispatchWorkerConfigClient omits type and supports API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDispatchWorkerConfigClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ type: "default", mcp_config: "{}", instructions: "Register worker", server_url: "http://localhost:9090/mcp/v1" }); } });
  const result = await client.get();
  assertEqual(result.type, "default");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workers/config");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("DispatchWorkerConfigClient exposes dispatch-worker-config nested gateway errors", async () => {
  const client = createDispatchWorkerConfigClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DISPATCH_CONFIG", message: "nested dispatch worker config failure" } }, { status: 400 }) });
  try { await client.get("bad"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof DispatchWorkerConfigClientError); assertEqual(error.name, "DispatchClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "DISPATCH_CONFIG", message: "nested dispatch worker config failure" } }); assertEqual(error.message, "nested dispatch worker config failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
