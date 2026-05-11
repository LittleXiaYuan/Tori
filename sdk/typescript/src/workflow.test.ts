import { createWorkflowClient, WorkflowClientError, WorkflowDefinition } from "./workflow";

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

test("WorkflowClient lists workflows with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowClient({
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

test("WorkflowClient gets saves and deletes definitions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowClient({
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

test("WorkflowClient runs workflows and reads instances", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const instance = { id: "inst1", definition_id: "wf1", status: "pending", variables: { topic: "sdk" } };
  const client = createWorkflowClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/run")) return jsonResponse({ status: "accepted", instance_id: "inst1", instance }, { status: 202 });
      if (String(url).includes("id=inst1")) return jsonResponse({ ...instance, status: "running" });
      return jsonResponse({ instances: [instance], total: 1 });
    },
  });

  const run = await client.run({ definition_id: "wf1", variables: { topic: "sdk" } });
  const listed = await client.instances();
  const fetched = await client.getInstance("inst1");

  assertEqual(run.status, "accepted");
  assertEqual(run.instance_id, "inst1");
  assertEqual(listed.total, 1);
  assertEqual(fetched.status, "running");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/run");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ definition_id: "wf1", variables: { topic: "sdk" } }));
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/workflows/instances?id=inst1");
});

test("WorkflowClient cancels workflow instances", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "cancelling", instance_id: "inst1" });
    },
  });

  const result = await client.cancel({ instance_id: "inst1" });

  assertEqual(result.status, "cancelling");
  assertEqual(result.instance_id, "inst1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/cancel");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ instance_id: "inst1" }));
});

test("WorkflowClient throws WorkflowClientError with parsed and text bodies", async () => {
  const jsonClient = createWorkflowClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "definition_id is required" }, { status: 400 }),
  });

  try {
    await jsonClient.run({ definition_id: "" });
    throw new Error("expected run to reject");
  } catch (error) {
    assert(error instanceof WorkflowClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "definition_id is required" });
    assertEqual(error.message, "definition_id is required");
  }


  const nestedClient = createWorkflowClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "workflow definition id is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.run({ definition_id: "" });
    throw new Error("expected run to reject");
  } catch (error) {
    assert(error instanceof WorkflowClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "workflow definition id is required");
  }

  const textClient = createWorkflowClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("workflow engine not available", { status: 404 }),
  });

  try {
    await textClient.list();
    throw new Error("expected list to reject");
  } catch (error) {
    assert(error instanceof WorkflowClientError);
    assertEqual(error.status, 404);
    assertEqual(error.body, "workflow engine not available");
    assertEqual(error.message, "workflow engine not available");
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
