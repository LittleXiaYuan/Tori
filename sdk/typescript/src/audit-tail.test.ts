import { createAuditTailClient, AuditTailClientError } from "./audit-tail";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AuditTailClient tails audit chain with bearer token and filters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditTailClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ records: [{ id: "a1", type: "task", actor: "u1" }], count: 1 }); } });
  const result = await client.tail({ n: 20, type: "task", actor: "u1" });
  assertEqual(result.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/audit/tail?n=20&type=task&actor=u1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("AuditTailClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditTailClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ records: [], count: 0 }); } });
  await client.tail();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/audit/tail");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("AuditTailClient exposes nested tail errors", async () => {
  const client = createAuditTailClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "AUDIT_TAIL", message: "audit tail denied" } }, { status: 403 }) });
  try { await client.tail({ n: 1 }); throw new Error("expected tail to reject"); } catch (error) { assert(error instanceof AuditTailClientError); assertEqual(error.name, "AuditClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "AUDIT_TAIL", message: "audit tail denied" } }); assertEqual(error.message, "audit tail denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
