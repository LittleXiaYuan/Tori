import { createLoRAConfigClient, LoRAConfigClientError } from "./lora-config";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRAConfigClient reads and updates config with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAConfigClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ config: { min_samples: 8 }, status: "ok" }); } });
  assertEqual((await client.get()).config.min_samples, 8);
  assertEqual((await client.update({ min_samples: 12 }, "PATCH")).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/config");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(calls[1]?.init?.method, "PATCH");
  assertEqual(new Headers(calls[1]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { min_samples: 12 });
});

test("LoRAConfigClient defaults update to PUT and supports API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAConfigClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ config: { base_model: "qwen" } }); } });
  await client.update({ base_model: "qwen" });
  assertEqual(calls[0]?.init?.method, "PUT");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("LoRAConfigClient exposes nested config errors", async () => {
  const client = createLoRAConfigClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_CONFIG", message: "invalid lora config" } }, { status: 400 }) });
  try { await client.update({ min_samples: -1 }); throw new Error("expected update to reject"); } catch (error) { assert(error instanceof LoRAConfigClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "LORA_CONFIG", message: "invalid lora config" } }); assertEqual(error.message, "invalid lora config"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
