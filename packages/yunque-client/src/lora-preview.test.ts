import { createLoRAPreviewClient, LoRAPreviewClientError } from "./lora-preview";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("LoRAPreviewClient reads tenant preview with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAPreviewClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ preview: { ready: true, tenant_id: "t1", sample_count: 12 } }); } });
  const result = await client.preview({ tenant_id: "t1" });
  assertEqual(result.preview.ready, true);
  assertEqual(result.preview.sample_count, 12);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/preview?tenant_id=t1");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("LoRAPreviewClient supports API key auth without tenant query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAPreviewClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ preview: { ready: false } }); } });
  await client.preview();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/preview");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("LoRAPreviewClient exposes nested preview errors", async () => {
  const client = createLoRAPreviewClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "LORA_PREVIEW", message: "nested lora preview failure" } }, { status: 503 }) });
  try { await client.preview(); throw new Error("expected preview to reject"); } catch (error) { assert(error instanceof LoRAPreviewClientError); assertEqual(error.name, "LoRAClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "LORA_PREVIEW", message: "nested lora preview failure" } }); assertEqual(error.message, "nested lora preview failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
