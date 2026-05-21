import { createUsageClient, UsageClientError } from "./usage";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("UsageClient reads usage through usage facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createUsageClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ tenant_id: "t1", chat_calls: 3, tokens_used: 128 }); } });

  const result = await client.usage();

  assertEqual(result.tenant_id, "t1");
  assertEqual(result.chat_calls, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/usage");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("UsageClient sets quota without importing generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createUsageClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });

  const result = await client.setQuota({ tenant_id: "t1", quota: { max_chat_calls: 20, max_tokens_per_day: 1000 } });

  assertEqual(result.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/quota");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ tenant_id: "t1", quota: { max_chat_calls: 20, max_tokens_per_day: 1000 } }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("UsageClient exposes usage-named errors", async () => {
  const client = createUsageClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "quota denied" } }, { status: 403 }) });

  try {
    await client.usage();
    throw new Error("expected usage to reject");
  } catch (error) {
    assert(error instanceof UsageClientError);
    assertEqual(error.status, 403);
    assertEqual(error.message, "quota denied");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
