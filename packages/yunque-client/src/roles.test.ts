import { createRolesClient, RolesClientError } from "./roles";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("RolesClient lists and creates roles through roles facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRolesClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "ops", name: "Ops", permissions: [] }); return jsonResponse({ roles: [{ id: "viewer", name: "Viewer", permissions: [] }], total: 1 }); } });

  const list = await client.list();
  const created = await client.create({ id: "ops", name: "Ops", permissions: [] });

  assertEqual(list.total, 1);
  assertEqual(created.id, "ops");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rbac/roles");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ id: "ops", name: "Ops", permissions: [] }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("RolesClient assigns revokes deletes and reads my roles without generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createRolesClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("my-roles")) return jsonResponse({ subject_id: "u1", roles: [], total: 0 }); if (init?.method === "DELETE") return jsonResponse({ deleted: "ops" }); return jsonResponse({ status: "assigned", subject_id: "u1", role_id: "ops" }); } });

  await client.assign({ subject_id: "u1", role_id: "ops" });
  await client.revoke({ subject_id: "u1", role_id: "ops" });
  const mine = await client.mine();
  const deleted = await client.delete("ops");

  assertEqual(mine.subject_id, "u1");
  assertEqual(deleted.deleted, "ops");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rbac/assign");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/rbac/revoke");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/rbac/my-roles");
  assertEqual(calls[3]?.url, "http://localhost:9090/v1/rbac/roles?id=ops");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("RolesClient exposes roles-named errors", async () => {
  const client = createRolesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "role denied" } }, { status: 403 }) });

  try {
    await client.list();
    throw new Error("expected list to reject");
  } catch (error) {
    assert(error instanceof RolesClientError);
    assertEqual(error.status, 403);
    assertEqual(error.message, "role denied");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
