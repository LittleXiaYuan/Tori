import { createTaskCreateClient, TaskCreateClientError } from "./task-create";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskCreateClient creates task requests with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskCreateClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ id: "task-1", description: "推进 Planner", status: "pending" }, { status: 201 });
    },
  });

  const task = await client.create({ title: "Planner", description: "推进 Planner", constraints: { max_steps: 8, risk_level: "low" } });

  assertEqual(task.id, "task-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ title: "Planner", description: "推进 Planner", constraints: { max_steps: 8, risk_level: "low" } }));
});

test("TaskCreateClient createFromDescription offers compact API with API key auth", async () => {
  const client = createTaskCreateClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (_url, init) => {
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      assertEqual(init?.body, JSON.stringify({ title: "SDK", constraints: { priority: "high" }, description: "拆 SDK 增量包" }));
      return jsonResponse({ id: "task-2", description: "拆 SDK 增量包" });
    },
  });

  const task = await client.createFromDescription("拆 SDK 增量包", { title: "SDK", constraints: { priority: "high" } });

  assertEqual(task.id, "task-2");
});

test("TaskCreateClient exposes task-create-named nested gateway errors", async () => {
  const client = createTaskCreateClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested task create failure" } }, { status: 400 }),
  });

  try {
    await client.createFromDescription("");
    throw new Error("expected createFromDescription to reject");
  } catch (error) {
    assert(error instanceof TaskCreateClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "nested task create failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
