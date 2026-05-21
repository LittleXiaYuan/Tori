import { createLoRATriggerClient, LoRATriggerClientError } from "./lora-trigger";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRATriggerClient triggers training with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRATriggerClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok", tenant_id: "tenant-1" }); } });
  assertEqual((await client.trigger({ tenant_id: "tenant-1" })).tenant_id, "tenant-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/trigger");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { tenant_id: "tenant-1" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("LoRATriggerClient supports default body and API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRATriggerClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.trigger();
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("LoRATriggerClient exposes nested trigger errors", async () => {
  const client = createLoRATriggerClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_TRIGGER", message: "nested lora trigger failure" } }, { status: 400 }) });
  try { await client.trigger({}); throw new Error("expected trigger to reject"); } catch (error) { assert(error instanceof LoRATriggerClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "LORA_TRIGGER", message: "nested lora trigger failure" } }); assertEqual(error.message, "nested lora trigger failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
