import { createResourceStateClient, ResourceStateClientError } from "./resource-state";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ResourceStateClient lists resources with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createResourceStateClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse([{ id: "r1", path: "sdk/typescript", type: "repo" }]); } });
  const resources = await client.list();
  assertEqual(resources[0]?.path, "sdk/typescript");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/state/resources");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ResourceStateClient tracks and releases resources with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createResourceStateClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ status: "released" }); return jsonResponse({ status: "tracked" }); } });
  const tracked = await client.track({ path: "sdk/typescript", type: "repo" });
  const released = await client.release("r1");
  assertEqual(tracked.status, "tracked");
  assertEqual(released.status, "released");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/state/resources");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/state/resources?id=r1");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { path: "sdk/typescript", type: "repo" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ResourceStateClient exposes resource-state nested gateway errors", async () => {
  const client = createResourceStateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested resource failure" } }, { status: 400 }) });
  try { await client.track({ path: "" }); throw new Error("expected track to reject"); } catch (error) { assert(error instanceof ResourceStateClientError); assertEqual(error.name, "StateClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested resource failure" } }); assertEqual(error.message, "nested resource failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
