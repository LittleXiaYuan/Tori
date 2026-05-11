import { createRBACClient, RBACClientError } from "./rbac";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("RBACClient lists, creates, and deletes roles with bearer auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const customRole = { id: "ops", name: "Ops", description: "Operate tasks", permissions: [{ resource: "tasks", action: "execute" }] };
  const client = createRBACClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ ...customRole, created_at: "2026-05-11T00:00:00Z" }, { status: 201 }); if (init?.method === "DELETE") return jsonResponse({ deleted: "ops" }); return jsonResponse({ roles: [{ id: "viewer", name: "Viewer", permissions: [{ resource: "chat", action: "read" }], is_built_in: true }], total: 1 }); } });
  const roles = await client.roles(); const created = await client.createRole(customRole); const deleted = await client.deleteRole("ops");
  assertEqual(roles.total, 1); assertEqual(created.id, "ops"); assertEqual(deleted.deleted, "ops"); assertEqual(calls[2]?.url, "http://localhost:9090/v1/rbac/roles?id=ops"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("RBACClient assigns and revokes subject roles with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRBACClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/assign")) return jsonResponse({ status: "assigned", subject_id: "user-1", role_id: "operator" }); return jsonResponse({ status: "revoked", subject_id: "user-1", role_id: "operator" }); } });
  const assigned = await client.assignRole({ subject_id: "user-1", role_id: "operator", tenant_id: "tenant-a" }); const revoked = await client.revokeRole({ subject_id: "user-1", role_id: "operator", tenant_id: "tenant-a" });
  assertEqual(assigned.status, "assigned"); assertEqual(revoked.status, "revoked"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/rbac/assign"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/rbac/revoke"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { subject_id: "user-1", role_id: "operator", tenant_id: "tenant-a" });
});

test("RBACClient checks permissions and reads current subject roles", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRBACClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/check")) return jsonResponse({ allowed: true, subject_id: "tenant-a", resource: "tasks", action: "write" }); return jsonResponse({ subject_id: "tenant-a", roles: [{ id: "operator", name: "Operator", permissions: [] }], total: 1 }); } });
  const result = await client.check({ resource: "tasks", action: "write" }); const mine = await client.myRoles();
  assertEqual(result.allowed, true); assertEqual(result.resource, "tasks"); assertEqual(mine.roles[0]?.id, "operator"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/rbac/check"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/rbac/my-roles"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { resource: "tasks", action: "write" });
});

test("RBACClient throws RBACClientError with parsed and text bodies", async () => {
  const jsonClient = createRBACClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "subject_id and role_id are required" }, { status: 400 }) });
  try { await jsonClient.assignRole({ subject_id: "", role_id: "" }); throw new Error("expected assignRole to reject"); } catch (error) { assert(error instanceof RBACClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "subject_id and role_id are required" }); assertEqual(error.message, "subject_id and role_id are required"); }
  const textClient = createRBACClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET/POST/DELETE only", { status: 405 }) });
  try { await textClient.roles(); throw new Error("expected roles to reject"); } catch (error) { assert(error instanceof RBACClientError); assertEqual(error.status, 405); assertEqual(error.body, "GET/POST/DELETE only"); assertEqual(error.message, "GET/POST/DELETE only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
