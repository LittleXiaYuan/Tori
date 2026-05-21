import { createAuditVerifyClient, AuditVerifyClientError } from "./audit-verify";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AuditVerifyClient verifies chain and reads stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditVerifyClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return String(url).endsWith("/stats") ? jsonResponse({ events: 12 }) : jsonResponse({ valid: true, checked: 12 }); } });
  assertEqual((await client.verify()).valid, true);
  assertEqual((await client.stats()).events, 12);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/audit/verify");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/audit/stats");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("AuditVerifyClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditVerifyClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ valid: true }); } });
  await client.verify();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("AuditVerifyClient exposes nested verify errors", async () => {
  const client = createAuditVerifyClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "AUDIT_VERIFY", message: "audit verify failed" } }, { status: 500 }) });
  try { await client.verify(); throw new Error("expected verify to reject"); } catch (error) { assert(error instanceof AuditVerifyClientError); assertEqual(error.name, "AuditClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "AUDIT_VERIFY", message: "audit verify failed" } }); assertEqual(error.message, "audit verify failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
