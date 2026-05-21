import { createApprovalHistoryClient, ApprovalHistoryClientError } from "./approval-history";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ApprovalHistoryClient lists approval history with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalHistoryClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ approvals: [{ id: "a1", status: "approved" }], total: 1 }); } });
  const result = await client.list("approved");
  assertEqual(result.total, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals?status=approved&history=true");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ApprovalHistoryClient supports API key auth and all-history query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalHistoryClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ approvals: [], total: 0 }); } });
  await client.list();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals?history=true");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ApprovalHistoryClient exposes nested history errors", async () => {
  const client = createApprovalHistoryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "HISTORY_DENIED", message: "approval history denied" } }, { status: 403 }) });
  try { await client.list("denied"); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ApprovalHistoryClientError); assertEqual(error.name, "ApprovalsClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "HISTORY_DENIED", message: "approval history denied" } }); assertEqual(error.message, "approval history denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
