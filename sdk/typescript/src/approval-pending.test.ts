import { createApprovalPendingClient, ApprovalPendingClientError } from "./approval-pending";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ApprovalPendingClient lists pending approvals with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalPendingClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ approvals: [{ id: "a1", status: "pending" }], total: 1 }); } });
  const result = await client.list();
  assertEqual(result.total, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals?status=pending");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ApprovalPendingClient approves denies and decides with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalPendingClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.approve("a1")).status, "ok");
  assertEqual((await client.deny("a2", "unsafe")).status, "ok");
  assertEqual((await client.decide("a3", "allow_once")).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals/approve");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/approvals/deny");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/approvals/decide");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "a2", reason: "unsafe" });
  assertEqual(new Headers(calls[2]?.init?.headers).get("x-api-key"), "key");
});

test("ApprovalPendingClient exposes nested pending errors", async () => {
  const client = createApprovalPendingClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "PENDING_DENIED", message: "pending approval denied" } }, { status: 403 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ApprovalPendingClientError); assertEqual(error.name, "ApprovalsClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "PENDING_DENIED", message: "pending approval denied" } }); assertEqual(error.message, "pending approval denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
