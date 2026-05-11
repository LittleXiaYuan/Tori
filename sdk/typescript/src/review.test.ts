import { createReviewClient, ReviewClientError } from "./review";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ReviewClient reads review gate status through review facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReviewClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ enabled: true, trust_enabled: true, status: "ready" }); } });

  const result = await client.status();

  assertEqual(result.enabled, true);
  assertEqual(result.trust_enabled, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/review/status");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ReviewClient exposes review-named errors", async () => {
  const client = createReviewClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "review gate unavailable" } }, { status: 503 }) });

  try {
    await client.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof ReviewClientError);
    assertEqual(error.status, 503);
    assertEqual(error.message, "review gate unavailable");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
