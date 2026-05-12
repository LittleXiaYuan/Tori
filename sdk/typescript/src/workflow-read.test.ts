import { createWorkflowReadClient, WorkflowReadClientError, WorkflowDefinition } from "./workflow-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
const demoWorkflow: WorkflowDefinition = { id: "wf1", name: "Review flow", nodes: [], edges: [] };

test("WorkflowReadClient lists definitions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ workflows: [demoWorkflow], total: 1 }); } });
  const result = await client.list();
  assertEqual(result.total, 1); assertEqual(result.workflows[0]?.id, "wf1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("WorkflowReadClient gets definitions with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowReadClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse(demoWorkflow); } });
  const result = await client.get("wf1");
  assertEqual(result.name, "Review flow"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows?id=wf1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("WorkflowReadClient exposes nested read errors", async () => {
  const client = createWorkflowReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested workflow read failure" } }, { status: 404 }) });
  try { await client.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof WorkflowReadClientError); assertEqual(error.name, "WorkflowClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "nested workflow read failure" } }); assertEqual(error.message, "nested workflow read failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
