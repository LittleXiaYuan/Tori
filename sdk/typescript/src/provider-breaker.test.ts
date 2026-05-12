import { createProviderBreakerClient, ProviderBreakerClientError } from "./provider-breaker";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ProviderBreakerClient resets breakers with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderBreakerClient({ baseUrl: "http://localhost:9090/", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, reset_count: 2 }); } });
  const result = await client.reset();
  assertEqual(result.ok, true); assertEqual(result.reset_count, 2); assertEqual(calls[0]?.url, "http://localhost:9090/api/breaker/reset"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ProviderBreakerClient exposes nested breaker errors", async () => {
  const client = createProviderBreakerClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FORBIDDEN", message: "breaker reset denied" } }, { status: 403 }) });
  try { await client.reset(); throw new Error("expected reset to reject"); } catch (error) { assert(error instanceof ProviderBreakerClientError); assertEqual(error.name, "ProvidersClientError"); assertEqual(error.status, 403); assertEqual(error.message, "breaker reset denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
