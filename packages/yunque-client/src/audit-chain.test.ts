import { createAuditChainClient, AuditChainClientError } from "./audit-chain";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AuditChainClient tails chain with bearer token and filters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditChainClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ records: [{ id: "r1" }], count: 1 }); } });
  assertEqual((await client.tail({ n: 10, type: "system", actor: "tenant" })).count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/audit/tail?n=10&type=system&actor=tenant");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("AuditChainClient verifies and reads stats with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditChainClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/stats")) return jsonResponse({ total: 12 }); return jsonResponse({ valid: true, checked: 12 }); } });
  assertEqual((await client.verify()).valid, true);
  assertEqual(((await client.stats()) as { total?: number }).total, 12);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/audit/verify");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/audit/stats");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("AuditChainClient exposes nested chain errors", async () => {
  const client = createAuditChainClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "AUDIT_CHAIN", message: "chain failed" } }, { status: 503 }) });
  try { await client.verify(); throw new Error("expected verify to reject"); } catch (error) { assert(error instanceof AuditChainClientError); assertEqual(error.name, "AuditClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "AUDIT_CHAIN", message: "chain failed" } }); assertEqual(error.message, "chain failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
