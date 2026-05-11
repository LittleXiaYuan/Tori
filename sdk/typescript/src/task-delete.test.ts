import { createTaskDeleteClient, TaskDeleteClientError } from "./task-delete";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskDeleteClient deletes tasks with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskDeleteClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ deleted: "task-1" });
    },
  });

  const result = await client.delete("task-1");

  assertEqual(result.deleted, "task-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks?id=task-1");
  assertEqual(calls[0]?.init?.method, "DELETE");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TaskDeleteClient supports API key auth", async () => {
  const client = createTaskDeleteClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      assertEqual(String(url), "http://localhost:9090/v1/tasks?id=task-2");
      assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123");
      return jsonResponse({ deleted: "task-2" });
    },
  });

  const result = await client.delete("task-2");

  assertEqual(result.deleted, "task-2");
});

test("TaskDeleteClient exposes task-delete-named nested gateway errors", async () => {
  const client = createTaskDeleteClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { message: "nested task delete failure" } }, { status: 404 }),
  });

  try {
    await client.delete("missing");
    throw new Error("expected delete to reject");
  } catch (error) {
    assert(error instanceof TaskDeleteClientError);
    assertEqual(error.status, 404);
    assertEqual(error.message, "nested task delete failure");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
