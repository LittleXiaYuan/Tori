import { createLoRARollbackClient, LoRARollbackClientError } from "./lora-rollback";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRARollbackClient rollbacks with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRARollbackClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.rollback()).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/rollback");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("LoRARollbackClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRARollbackClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.rollback();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("LoRARollbackClient exposes nested rollback errors", async () => {
  const client = createLoRARollbackClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_ROLLBACK", message: "nested lora rollback failure" } }, { status: 409 }) });
  try { await client.rollback(); throw new Error("expected rollback to reject"); } catch (error) { assert(error instanceof LoRARollbackClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: { code: "LORA_ROLLBACK", message: "nested lora rollback failure" } }); assertEqual(error.message, "nested lora rollback failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
