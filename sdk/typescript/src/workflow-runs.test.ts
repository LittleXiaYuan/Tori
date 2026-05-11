import { createWorkflowRunsClient, WorkflowRunsClientError } from "./workflow-runs";

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

test("WorkflowRunsClient runs workflow definitions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const instance = { id: "inst1", definition_id: "wf1", status: "pending", variables: { topic: "sdk" } };
  const client = createWorkflowRunsClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "accepted", instance_id: "inst1", instance }, { status: 202 });
    },
  });

  const result = await client.runDefinition("wf1", { topic: "sdk" });

  assertEqual(result.instance_id, "inst1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/run");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ definition_id: "wf1", variables: { topic: "sdk" } }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("WorkflowRunsClient lists and gets workflow instances with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const instance = { id: "inst1", definition_id: "wf1", status: "running" };
  const client = createWorkflowRunsClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("id=inst1")) return jsonResponse(instance);
      return jsonResponse({ instances: [instance], total: 1 });
    },
  });

  const listed = await client.list();
  const fetched = await client.get("inst1");

  assertEqual(listed.total, 1);
  assertEqual(fetched.status, "running");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/instances");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/workflows/instances?id=inst1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("WorkflowRunsClient cancels workflow instances", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowRunsClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "cancelling", instance_id: "inst1" });
    },
  });

  const result = await client.cancel("inst1");

  assertEqual(result.status, "cancelling");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/cancel");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ instance_id: "inst1" }));
});

test("WorkflowRunsClient exposes workflow-runs nested gateway errors", async () => {
  const client = createWorkflowRunsClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested run failure" } }, { status: 400 }),
  });

  try {
    await client.run({ definition_id: "" });
    throw new Error("expected run to reject");
  } catch (error) {
    assert(error instanceof WorkflowRunsClientError);
    assertEqual(error.name, "WorkflowClientError");
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested run failure" } });
    assertEqual(error.message, "nested run failure");
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
