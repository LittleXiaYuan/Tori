import { createLoRAObserveClient, LoRAObserveClientError } from "./lora-observe";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRAObserveClient reads status with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAObserveClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ active_model: "base", rolling_success_rate: 0.9 }); } });
  assertEqual((await client.status()).active_model, "base");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/status");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("LoRAObserveClient reads history summary preview and evolution with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAObserveClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.includes("/history")) return jsonResponse({ records: [{ adapter: "a1" }], count: 1 }); if (path.includes("/summary")) return jsonResponse({ summary: { best_score: 0.9 } }); if (path.includes("/preview")) return jsonResponse({ preview: { ready: true, tenant_id: "tenant-1" } }); return jsonResponse({ state: { rolling_success_rate: 0.75 } }); } });
  assertEqual((await client.history()).count, 1);
  assertEqual(((await client.summary()).summary as { best_score?: number }).best_score, 0.9);
  assertEqual((await client.preview({ tenant_id: "tenant-1" })).preview.ready, true);
  assertEqual(((await client.evolution()).state as { rolling_success_rate?: number }).rolling_success_rate, 0.75);
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/lora/preview?tenant_id=tenant-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("LoRAObserveClient exposes nested observe errors", async () => {
  const client = createLoRAObserveClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_OBSERVE", message: "observe failed" } }, { status: 500 }) });
  try { await client.status(); throw new Error("expected status to reject"); } catch (error) { assert(error instanceof LoRAObserveClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "LORA_OBSERVE", message: "observe failed" } }); assertEqual(error.message, "observe failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
