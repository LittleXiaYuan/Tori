import { createAuditTrailClient, AuditTrailClientError } from "./audit-trail";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AuditTrailClient reads task audit trail with bearer token and filters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditTrailClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entries: [{ operation: "nl_config", result: "ok" }], count: 1 }); } });
  assertEqual((await client.trail({ date: "2026-05-11", type: "nl_config" })).count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/audit/trail?date=2026-05-11&type=nl_config");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("AuditTrailClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditTrailClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ entries: [], count: 0 }); } });
  assertEqual((await client.trail()).count, 0);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/audit/trail");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("AuditTrailClient exposes nested trail errors", async () => {
  const client = createAuditTrailClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "AUDIT_TRAIL", message: "trail failed" } }, { status: 400 }) });
  try { await client.trail({ date: "" }); throw new Error("expected trail to reject"); } catch (error) { assert(error instanceof AuditTrailClientError); assertEqual(error.name, "AuditClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "AUDIT_TRAIL", message: "trail failed" } }); assertEqual(error.message, "trail failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
