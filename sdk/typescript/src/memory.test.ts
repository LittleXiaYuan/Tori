import { createMemoryClient, MemoryClientError } from "./memory";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
}

const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];

function test(name: string, fn: () => Promise<void> | void): void {
  tests.push({ name, fn });
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

test("MemoryClient reads stats with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createMemoryClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ short: 1, mid: 2, long: 3 });
    },
  });

  const stats = await client.stats();

  assertEqual(stats.long, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/memory/stats");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("MemoryClient searches memories with POST body", async () => {
  const calls: { init?: RequestInit }[] = [];
  const client = createMemoryClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      calls.push({ init });
      return jsonResponse({ results: [{ key: "k1", value: "v1" }], count: 1 });
    },
  });

  const result = await client.search({ query: "偏好", limit: 5, layer: "all" });

  assertEqual(result.count, 1);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ query: "偏好", limit: 5, layer: "all" }));
});

test("MemoryClient normalizes content to value when adding memory", async () => {
  const calls: { init?: RequestInit }[] = [];
  const client = createMemoryClient({
    baseUrl: "http://localhost:9090",
    fetch: async (_url, init) => {
      calls.push({ init });
      return jsonResponse({ status: "ok" }, { status: 201 });
    },
  });

  const result = await client.add({ layer: "long", content: "用户偏好简洁回答", source: "sdk-test" });

  assertEqual(result.status, "ok");
  assertEqual(
    calls[0]?.init?.body,
    JSON.stringify({ layer: "long", content: "用户偏好简洁回答", source: "sdk-test", value: "用户偏好简洁回答" }),
  );
});

test("MemoryClient throws MemoryClientError with parsed body", async () => {
  const client = createMemoryClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "value is required" }, { status: 400 }),
  });

  try {
    await client.add({ layer: "mid" });
    throw new Error("expected add to reject");
  } catch (error) {
    assert(error instanceof MemoryClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "value is required" });
    assertEqual(error.message, "value is required");
  }

  const nestedClient = createMemoryClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "memory query is required" } }, { status: 400 }),
  });
  try {
    await nestedClient.search({ query: "" });
    throw new Error("expected nested search to reject");
  } catch (error) {
    assert(error instanceof MemoryClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "memory query is required");
  }
});

let failures = 0;
for (const { name, fn } of tests) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failures > 0) {
  process.exitCode = 1;
} else {
  console.log(`1..${tests.length}`);
}
