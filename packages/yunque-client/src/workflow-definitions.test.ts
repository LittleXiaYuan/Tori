import { createWorkflowDefinitionsClient, WorkflowDefinitionsClientError, WorkflowDefinition } from "./workflow-definitions";

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

const demoWorkflow: WorkflowDefinition = {
  id: "wf1",
  name: "Review flow",
  version: 1,
  nodes: [{ id: "n1", name: "Review", type: "llm", position: { x: 0, y: 0 } }],
  edges: [],
};

test("WorkflowDefinitionsClient lists definitions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowDefinitionsClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ workflows: [demoWorkflow], total: 1 });
    },
  });

  const result = await client.list();

  assertEqual(result.total, 1);
  assertEqual(result.workflows[0]?.name, "Review flow");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("WorkflowDefinitionsClient gets saves and deletes definitions with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowDefinitionsClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (init?.method === "POST") return jsonResponse({ ...demoWorkflow, id: "wf2" }, { status: 201 });
      if (init?.method === "DELETE") return jsonResponse({ deleted: "wf2" });
      return jsonResponse(demoWorkflow);
    },
  });

  const fetched = await client.get("wf1");
  const saved = await client.save({ name: "Review flow", nodes: [], edges: [] });
  const deleted = await client.delete("wf2");

  assertEqual(fetched.id, "wf1");
  assertEqual(saved.id, "wf2");
  assertEqual(deleted.deleted, "wf2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows?id=wf1");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ name: "Review flow", nodes: [], edges: [] }));
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/workflows?id=wf2");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("WorkflowDefinitionsClient exposes workflow-definitions nested gateway errors", async () => {
  const client = createWorkflowDefinitionsClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested definition failure" } }, { status: 400 }),
  });

  try {
    await client.get("");
    throw new Error("expected get to reject");
  } catch (error) {
    assert(error instanceof WorkflowDefinitionsClientError);
    assertEqual(error.name, "WorkflowClientError");
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested definition failure" } });
    assertEqual(error.message, "nested definition failure");
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
