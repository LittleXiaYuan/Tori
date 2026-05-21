import { createTriggerDefinitionsClient, TriggerDefinitionsClientError } from "./trigger-definitions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TriggerDefinitionsClient lists and gets v2 triggers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggerDefinitionsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (new URL(String(url)).searchParams.get("id")) return jsonResponse({ id: "v2-1", name: "daily" }); return jsonResponse({ triggers: [{ id: "v2-1", name: "daily" }], total: 1 }); } });
  assertEqual((await client.list({ tenantId: "default", type: "event", status: "enabled" })).total, 1); assertEqual((await client.get("v2-1")).name, "daily"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers/v2?tenant_id=default&type=event&status=enabled"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/triggers/v2?id=v2-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TriggerDefinitionsClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggerDefinitionsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ triggers: [], total: 0 }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TriggerDefinitionsClient exposes nested definition errors", async () => {
  const client = createTriggerDefinitionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested trigger definition failure" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof TriggerDefinitionsClientError); assertEqual(error.name, "TriggersClientError"); assertEqual(error.status, 400); assertEqual(error.message, "nested trigger definition failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
