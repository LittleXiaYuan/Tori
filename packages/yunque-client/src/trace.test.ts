import { createTraceClient, TraceClientError } from "./trace";

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

test("TraceClient reads recent trace events with auth and query options", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTraceClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ count: 1, raw: false, events: [{ id: "evt-1", trace_id: "tr-1" }] });
    },
  });

  const result = await client.recent({ limit: 10 });

  assertEqual(result.count, 1);
  assertEqual(result.events[0]?.trace_id, "tr-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/recent?limit=10");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TraceClient reads trace events by trace id in raw mode", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTraceClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ trace_id: "tr/complex", count: 1, raw: true, events: [{ id: "evt-2", data: { raw: true } }] });
    },
  });

  const result = await client.byTraceId("tr/complex", { raw: true });

  assertEqual(result.raw, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/tr%2Fcomplex?raw=true");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TraceClient reads trace events by task id", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTraceClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ task_id: "task-1", count: 2, events: [{ id: "evt-3" }, { id: "evt-4" }] });
    },
  });

  const result = await client.byTaskId("task-1");

  assertEqual(result.task_id, "task-1");
  assertEqual(result.count, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/task/task-1");
});

test("TraceClient throws TraceClientError with text body", async () => {
  const client = createTraceClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("audit trail not available", { status: 503 }),
  });

  try {
    await client.recent();
    throw new Error("expected recent to reject");
  } catch (error) {
    assert(error instanceof TraceClientError);
    assertEqual(error.status, 503);
    assertDeepEqual(error.body, "audit trail not available");
    assertEqual(error.message, "audit trail not available");
  }

  const nestedClient = createTraceClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "trace id is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.byTraceId("");
    throw new Error("expected byTraceId to reject");
  } catch (error) {
    assert(error instanceof TraceClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "trace id is required");
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
