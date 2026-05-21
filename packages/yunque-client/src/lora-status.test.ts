import { createLoRAStatusClient, LoRAStatusClientError } from "./lora-status";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRAStatusClient reads status preview and evolution with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAStatusClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.includes("preview")) return jsonResponse({ preview: { ready: true, tenant_id: "t1" } }); if (path.includes("evolution")) return jsonResponse({ state: { phase: "eval" } }); return jsonResponse({ active_model: "adapter-a" }); } });
  assertEqual((await client.status()).active_model, "adapter-a");
  assertEqual((await client.preview({ tenant_id: "t1" })).preview.tenant_id, "t1");
  assertDeepEqual((await client.evolution()).state, { phase: "eval" });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/lora/preview?tenant_id=t1");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/lora/evolution");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("LoRAStatusClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAStatusClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ active_model: "base" }); } });
  await client.status();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("LoRAStatusClient exposes nested status errors", async () => {
  const client = createLoRAStatusClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_STATUS", message: "lora status unavailable" } }, { status: 503 }) });
  try { await client.status(); throw new Error("expected status to reject"); } catch (error) { assert(error instanceof LoRAStatusClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "LORA_STATUS", message: "lora status unavailable" } }); assertEqual(error.message, "lora status unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
