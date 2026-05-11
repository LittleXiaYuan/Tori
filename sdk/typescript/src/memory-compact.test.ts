import { createMemoryCompactClient, MemoryCompactClientError } from "./memory-compact";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("MemoryCompactClient compacts memory with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryCompactClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "ok", compacted: 4 });
    },
  });

  const result = await client.compact({ target_count: 100, decay_days: 30 });

  assertEqual(result.status, "ok");
  assertEqual(result.compacted, 4);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory/compact");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ target_count: 100, decay_days: 30 }));
});

test("MemoryCompactClient supports empty compact request and API key auth", async () => {
  const client = createMemoryCompactClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      assertEqual(init?.body, JSON.stringify({}));
      return jsonResponse({ status: "queued" });
    },
  });

  const result = await client.compact();

  assertEqual(result.status, "queued");
});

test("MemoryCompactClient exposes memory-compact-named nested gateway errors", async () => {
  const client = createMemoryCompactClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested memory compact failure" } }, { status: 409 }),
  });

  try {
    await client.compact();
    throw new Error("expected compact to reject");
  } catch (error) {
    assert(error instanceof MemoryCompactClientError);
    assertEqual(error.status, 409);
    assertEqual(error.message, "nested memory compact failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
