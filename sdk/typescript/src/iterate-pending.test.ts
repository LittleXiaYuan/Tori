import { createIteratePendingClient, IteratePendingClientError } from "./iterate-pending";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("IteratePendingClient lists pending proposals with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIteratePendingClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ proposals: [{ id: "p1", status: "pending" }], count: 1 }); } });
  const result = await client.list();
  assertEqual(result.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/iterate/proposals?status=pending");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("IteratePendingClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIteratePendingClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ proposals: [], count: 0 }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("IteratePendingClient exposes nested pending errors", async () => {
  const client = createIteratePendingClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "ITERATE_PENDING", message: "pending proposals unavailable" } }, { status: 503 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof IteratePendingClientError); assertEqual(error.name, "IterateClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "ITERATE_PENDING", message: "pending proposals unavailable" } }); assertEqual(error.message, "pending proposals unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
