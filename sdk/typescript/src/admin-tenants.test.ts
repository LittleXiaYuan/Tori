import { createAdminTenantsClient, AdminTenantsClientError } from "./admin-tenants";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AdminTenantsClient lists and creates tenants with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAdminTenantsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "t2", name: "team", api_key: "ya_test" }, { status: 201 }); return jsonResponse({ tenants: [{ id: "t1", name: "default" }], count: 1 }); } });
  const list = await client.listTenants();
  const created = await client.createTenant("team");
  assertEqual(list.count, 1);
  assertEqual(created.api_key, "ya_test");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tenants");
  assertEqual(calls[1]?.init?.method, "POST");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ name: "team" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("AdminTenantsClient exposes nested tenant errors", async () => {
  const client = createAdminTenantsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "TENANT", message: "tenant name is required" } }, { status: 400 }) });
  try { await client.createTenant(""); throw new Error("expected createTenant to reject"); } catch (error) { assert(error instanceof AdminTenantsClientError); assertEqual(error.name, "AdminClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "TENANT", message: "tenant name is required" } }); assertEqual(error.message, "tenant name is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
