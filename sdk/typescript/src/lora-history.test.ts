import { createLoRAHistoryClient, LoRAHistoryClientError } from "./lora-history";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRAHistoryClient reads history and summary with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAHistoryClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return String(url).includes("summary") ? jsonResponse({ summary: { best: "adapter-a" } }) : jsonResponse({ records: [{ id: "r1" }], count: 1 }); } });
  assertEqual((await client.history()).count, 1);
  assertDeepEqual((await client.summary()).summary, { best: "adapter-a" });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/history");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/lora/summary");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("LoRAHistoryClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAHistoryClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ records: [], count: 0 }); } });
  await client.history();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("LoRAHistoryClient exposes nested history errors", async () => {
  const client = createLoRAHistoryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_HISTORY", message: "history retention denied" } }, { status: 403 }) });
  try { await client.history(); throw new Error("expected history to reject"); } catch (error) { assert(error instanceof LoRAHistoryClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "LORA_HISTORY", message: "history retention denied" } }); assertEqual(error.message, "history retention denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
