import { createRoleBindingsClient, RoleBindingsClientError } from "./role-bindings";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("RoleBindingsClient assigns and revokes roles with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRoleBindingsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const body = JSON.parse(String(init?.body)); return jsonResponse({ status: String(url).includes("revoke") ? "revoked" : "assigned", subject_id: body.subject_id, role_id: body.role_id }); } });
  assertEqual((await client.assign({ subject_id: "u1", role_id: "admin", tenant_id: "t1" })).status, "assigned");
  assertEqual((await client.revoke({ subject_id: "u1", role_id: "admin" })).status, "revoked");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rbac/assign");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/rbac/revoke");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { subject_id: "u1", role_id: "admin", tenant_id: "t1" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("RoleBindingsClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRoleBindingsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "assigned", subject_id: "u1", role_id: "viewer" }); } });
  await client.assign({ subject_id: "u1", role_id: "viewer" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("RoleBindingsClient exposes nested binding errors", async () => {
  const client = createRoleBindingsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "RBAC_BIND", message: "role binding denied" } }, { status: 403 }) });
  try { await client.assign({ subject_id: "u1", role_id: "admin" }); throw new Error("expected assign to reject"); } catch (error) { assert(error instanceof RoleBindingsClientError); assertEqual(error.name, "RBACClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "RBAC_BIND", message: "role binding denied" } }); assertEqual(error.message, "role binding denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
