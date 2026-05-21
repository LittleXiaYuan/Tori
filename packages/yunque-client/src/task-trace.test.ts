import { createTaskTraceClient, TaskTraceClientError } from "./task-trace";

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

test("TaskTraceClient reads task trace events with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskTraceClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ task_id: "task-1", count: 2, events: [{ id: "evt-1" }, { id: "evt-2" }] });
    },
  });

  const result = await client.get("task-1");

  assertEqual(result.task_id, "task-1");
  assertEqual(result.count, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/task/task-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TaskTraceClient supports raw query and API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskTraceClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ task_id: "task/complex", count: 1, raw: true, events: [{ id: "evt-3", data: { raw: true } }] });
    },
  });

  const result = await client.get("task/complex", { raw: true });

  assertEqual(result.raw, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/trace/task/task%2Fcomplex?raw=true");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TaskTraceClient exposes task-trace nested gateway errors", async () => {
  const client = createTaskTraceClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested task trace failure" } }, { status: 400 }),
  });

  try {
    await client.get("");
    throw new Error("expected get to reject");
  } catch (error) {
    assert(error instanceof TaskTraceClientError);
    assertEqual(error.name, "TraceClientError");
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested task trace failure" } });
    assertEqual(error.message, "nested task trace failure");
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
