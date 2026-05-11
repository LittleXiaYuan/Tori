import { createTasksClient, TasksClientError } from "./tasks";

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

test("TasksClient creates tasks with constraints", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTasksClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ id: "task-1", description: "推进 Planner", status: "pending" }, { status: 201 });
    },
  });

  const task = await client.create({
    title: "Planner",
    description: "推进 Planner",
    constraints: { max_steps: 8, risk_level: "low" },
  });

  assertEqual(task.id, "task-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(
    calls[0]?.init?.body,
    JSON.stringify({ title: "Planner", description: "推进 Planner", constraints: { max_steps: 8, risk_level: "low" } }),
  );
});

test("TasksClient reads a single task with query id", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTasksClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ id: "task-2", status: "running" });
    },
  });

  const task = await client.get("task-2");

  assertEqual(task.status, "running");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks?id=task-2");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TasksClient sends lifecycle actions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTasksClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "accepted", task_id: "task-3" }, { status: 202 });
    },
  });

  const result = await client.run("task-3");

  assertEqual(result.task_id, "task-3");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/run");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "task-3" }));
});

test("TasksClient deletes tasks by id query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTasksClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ deleted: "task-4" });
    },
  });

  const result = await client.delete("task-4");

  assertEqual(result.deleted, "task-4");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks?id=task-4");
  assertEqual(calls[0]?.init?.method, "DELETE");
});

test("TasksClient throws TasksClientError with parsed body", async () => {
  const client = createTasksClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "task not found" }, { status: 404 }),
  });

  try {
    await client.get("missing");
    throw new Error("expected get to reject");
  } catch (error) {
    assert(error instanceof TasksClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "task not found" });
    assertEqual(error.message, "task not found");
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
