import { createApprovalDecisionsClient, ApprovalDecisionsClientError } from "./approval-decisions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ApprovalDecisionsClient approves denies and persists decisions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalDecisionsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/approve")) return jsonResponse({ status: "approved", id: "ap-1" }); if (String(url).endsWith("/deny")) return jsonResponse({ status: "denied", id: "ap-1" }); return jsonResponse({ status: "saved", id: "ap-1", decision: "allow_always" }); } });
  assertEqual((await client.approve("ap-1")).status, "approved"); assertEqual((await client.deny("ap-1", "unsafe")).status, "denied"); assertEqual((await client.decide("ap-1", "allow_always")).decision, "allow_always"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals/approve"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "ap-1", reason: "unsafe" }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ApprovalDecisionsClient exposes nested decision errors", async () => {
  const client = createApprovalDecisionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DENIED", message: "approval id is required" } }, { status: 400 }) });
  try { await client.approve(""); throw new Error("expected approve to reject"); } catch (error) { assert(error instanceof ApprovalDecisionsClientError); assertEqual(error.name, "ApprovalsClientError"); assertEqual(error.status, 400); assertEqual(error.message, "approval id is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
