import { createLoRAControlClient, LoRAControlClientError } from "./lora-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRAControlClient triggers training and rollback with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/trigger")) return jsonResponse({ status: "ok", tenant_id: "tenant-1" }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.trigger({ tenant_id: "tenant-1" })).tenant_id, "tenant-1");
  assertEqual((await client.rollback()).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/trigger");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/lora/rollback");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { tenant_id: "tenant-1" });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), {});
});

test("LoRAControlClient reads and updates config with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "GET") return jsonResponse({ config: { min_samples: 10 } }); return jsonResponse({ config: { min_samples: 12 }, status: "updated" }); } });
  assertEqual(((await client.config()).config as { min_samples?: number }).min_samples, 10);
  assertEqual((await client.updateConfig({ min_samples: 12 }, "PATCH")).status, "updated");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/config");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/lora/config");
  assertEqual(calls[1]?.init?.method, "PATCH");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { min_samples: 12 });
});

test("LoRAControlClient supports default trigger body and exposes nested control errors", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const okClient = createLoRAControlClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await okClient.trigger();
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});

  const errorClient = createLoRAControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_CONTROL", message: "control failed" } }, { status: 400 }) });
  try { await errorClient.trigger({}); throw new Error("expected trigger to reject"); } catch (error) { assert(error instanceof LoRAControlClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "LORA_CONTROL", message: "control failed" } }); assertEqual(error.message, "control failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
