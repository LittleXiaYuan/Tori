import { createWorkflowInstancesClient, WorkflowInstancesClientError } from "./workflow-instances";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("WorkflowInstancesClient lists workflow instances with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const instance = { id: "inst1", definition_id: "wf1", status: "running" };
  const client = createWorkflowInstancesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ instances: [instance], total: 1 }); } });
  const result = await client.list();
  assertEqual(result.total, 1); assertEqual(result.instances[0]?.id, "inst1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/instances"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("WorkflowInstancesClient gets workflow instances with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const instance = { id: "inst1", definition_id: "wf1", status: "running" };
  const client = createWorkflowInstancesClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse(instance); } });
  const result = await client.get("inst1");
  assertEqual(result.status, "running"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/instances?id=inst1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("WorkflowInstancesClient exposes nested instance errors", async () => {
  const client = createWorkflowInstancesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested workflow instance failure" } }, { status: 404 }) });
  try { await client.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof WorkflowInstancesClientError); assertEqual(error.name, "WorkflowClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "nested workflow instance failure" } }); assertEqual(error.message, "nested workflow instance failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
