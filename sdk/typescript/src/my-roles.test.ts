import { createMyRolesClient, MyRolesClientError } from "./my-roles";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("MyRolesClient reads current subject roles with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMyRolesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ subject_id: "u1", roles: [{ id: "viewer", name: "Viewer", permissions: [] }], total: 1 }); } });
  const result = await client.get();
  assertEqual(result.subject_id, "u1");
  assertEqual(result.total, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/rbac/my-roles");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("MyRolesClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMyRolesClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ subject_id: "api", roles: [], total: 0 }); } });
  await client.get();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("MyRolesClient exposes nested my-roles errors", async () => {
  const client = createMyRolesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "RBAC_ME", message: "my roles denied" } }, { status: 401 }) });
  try { await client.get(); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof MyRolesClientError); assertEqual(error.name, "RBACClientError"); assertEqual(error.status, 401); assertDeepEqual(error.body, { error: { code: "RBAC_ME", message: "my roles denied" } }); assertEqual(error.message, "my roles denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
