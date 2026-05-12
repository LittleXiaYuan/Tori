import { createLoRAEvolutionClient, LoRAEvolutionClientError } from "./lora-evolution";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRAEvolutionClient reads evolution state with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAEvolutionClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ state: { phase: "eval", generation: 3 } }); } });
  assertDeepEqual((await client.evolution()).state, { phase: "eval", generation: 3 });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/evolution");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("LoRAEvolutionClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAEvolutionClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ state: { phase: "idle" } }); } });
  await client.evolution();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("LoRAEvolutionClient exposes nested evolution errors", async () => {
  const client = createLoRAEvolutionClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_EVOLUTION", message: "nested lora evolution failure" } }, { status: 503 }) });
  try { await client.evolution(); throw new Error("expected evolution to reject"); } catch (error) { assert(error instanceof LoRAEvolutionClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "LORA_EVOLUTION", message: "nested lora evolution failure" } }); assertEqual(error.message, "nested lora evolution failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
