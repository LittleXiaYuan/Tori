import { createPermissionsClient, PermissionsClientError } from "./permissions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PermissionsClient checks permissions through permissions facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPermissionsClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ allowed: true, subject_id: "u1", resource: "knowledge", action: "read" }); } });

  const result = await client.check({ subject_id: "u1", resource: "knowledge", action: "read" });

  assertEqual(result.allowed, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rbac/check");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ subject_id: "u1", resource: "knowledge", action: "read" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("PermissionsClient reads current roles without generated SDK", async () => {
  const client = createPermissionsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { assertEqual(String(url), "http://localhost:9090/v1/rbac/my-roles"); assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123"); return jsonResponse({ subject_id: "u1", roles: [{ id: "viewer", name: "Viewer", permissions: [] }], total: 1 }); } });

  const result = await client.myRoles();

  assertEqual(result.total, 1);
  assertEqual(result.roles[0]?.id, "viewer");
});

test("PermissionsClient exposes permissions-named errors", async () => {
  const client = createPermissionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "permission denied" } }, { status: 403 }) });

  try {
    await client.check({ resource: "knowledge", action: "write" });
    throw new Error("expected check to reject");
  } catch (error) {
    assert(error instanceof PermissionsClientError);
    assertEqual(error.status, 403);
    assertEqual(error.message, "permission denied");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
