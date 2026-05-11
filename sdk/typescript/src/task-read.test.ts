import { createTaskReadClient, TaskReadClientError } from "./task-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskReadClient lists tasks with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskReadClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse([{ id: "task-1", status: "running" }]);
    },
  });

  const tasks = await client.list();

  assertEqual(tasks[0]?.id, "task-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TaskReadClient gets task details with API key auth", async () => {
  const client = createTaskReadClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      assertEqual(String(url), "http://localhost:9090/v1/tasks?id=task-2");
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      return jsonResponse({ id: "task-2", status: "completed", steps: [{ action: "test", status: "done" }] });
    },
  });

  const task = await client.get("task-2");

  assertEqual(task.status, "completed");
  assertEqual(task.steps?.[0]?.status, "done");
});

test("TaskReadClient exposes task-read-named nested gateway errors", async () => {
  const client = createTaskReadClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested task read failure" } }, { status: 404 }),
  });

  try {
    await client.get("missing");
    throw new Error("expected get to reject");
  } catch (error) {
    assert(error instanceof TaskReadClientError);
    assertEqual(error.status, 404);
    assertEqual(error.message, "nested task read failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
