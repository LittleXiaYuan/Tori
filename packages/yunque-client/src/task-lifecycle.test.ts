import { createTaskLifecycleClient, TaskLifecycleClientError } from "./task-lifecycle";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskLifecycleClient runs and pauses tasks with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskLifecycleClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "accepted", task_id: "task-1" }, { status: 202 });
    },
  });

  const run = await client.run("task-1");
  await client.pause("task-1");

  assertEqual(run.task_id, "task-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/run");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/tasks/pause");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "task-1" }));
});

test("TaskLifecycleClient supports generic action with API key auth", async () => {
  const client = createTaskLifecycleClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      assertEqual(String(url), "http://localhost:9090/v1/tasks/resume");
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      assertEqual(init?.body, JSON.stringify({ id: "task-2" }));
      return jsonResponse({ status: "accepted", task_id: "task-2" });
    },
  });

  const result = await client.action("resume", "task-2");

  assertEqual(result.task_id, "task-2");
});

test("TaskLifecycleClient exposes lifecycle-named nested gateway errors", async () => {
  const client = createTaskLifecycleClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested task lifecycle failure" } }, { status: 409 }),
  });

  try {
    await client.cancel("task-3");
    throw new Error("expected cancel to reject");
  } catch (error) {
    assert(error instanceof TaskLifecycleClientError);
    assertEqual(error.status, 409);
    assertEqual(error.message, "nested task lifecycle failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
