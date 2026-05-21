import { createMemoryStatsClient, MemoryStatsClientError } from "./memory-stats";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("MemoryStatsClient reads memory stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryStatsClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ short: 1, mid: 2, long: 3 });
    },
  });

  const stats = await client.stats();

  assertEqual(stats.short, 1);
  assertEqual(stats.mid, 2);
  assertEqual(stats.long, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory/stats");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("MemoryStatsClient supports API key auth", async () => {
  const client = createMemoryStatsClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      return jsonResponse({ short: 0, mid: 0, long: 1 });
    },
  });

  const stats = await client.stats();

  assertEqual(stats.long, 1);
});

test("MemoryStatsClient exposes memory-stats-named nested gateway errors", async () => {
  const client = createMemoryStatsClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested memory stats failure" } }, { status: 503 }),
  });

  try {
    await client.stats();
    throw new Error("expected stats to reject");
  } catch (error) {
    assert(error instanceof MemoryStatsClientError);
    assertEqual(error.status, 503);
    assertEqual(error.message, "nested memory stats failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
