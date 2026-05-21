import { createIterateDecisionsClient, IterateDecisionsClientError } from "./iterate-decisions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("IterateDecisionsClient approves and rejects with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIterateDecisionsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: String(url).includes("reject") ? "rejected" : "approved", id: JSON.parse(String(init?.body)).id }); } });
  assertEqual((await client.approve({ id: "p1" })).status, "approved");
  assertEqual((await client.reject({ id: "p2" })).status, "rejected");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/iterate/approve");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/iterate/reject");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "p1" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("IterateDecisionsClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIterateDecisionsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "approved" }); } });
  await client.approve({ id: "p1" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("IterateDecisionsClient exposes nested decision errors", async () => {
  const client = createIterateDecisionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "ITERATE_DECISION", message: "proposal decision denied" } }, { status: 403 }) });
  try { await client.reject({ id: "p1" }); throw new Error("expected reject to reject"); } catch (error) { assert(error instanceof IterateDecisionsClientError); assertEqual(error.name, "IterateClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "ITERATE_DECISION", message: "proposal decision denied" } }); assertEqual(error.message, "proposal decision denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
