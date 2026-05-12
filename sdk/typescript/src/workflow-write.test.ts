import { createWorkflowWriteClient, WorkflowWriteClientError, WorkflowDefinition } from "./workflow-write";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
const demoWorkflow: WorkflowDefinition = { id: "wf1", name: "Review flow", nodes: [], edges: [] };

test("WorkflowWriteClient saves definitions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowWriteClient({ baseUrl: "http://localhost:9090", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ...demoWorkflow, id: "wf2" }, { status: 201 }); } });
  const result = await client.save({ name: "Review flow", nodes: [], edges: [] });
  assertEqual(result.id, "wf2"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows"); assertEqual(calls[0]?.init?.method, "POST"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "Review flow", nodes: [], edges: [] }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("WorkflowWriteClient deletes definitions with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowWriteClient({ baseUrl: "http://localhost:9090/", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ deleted: "wf2" }); } });
  const result = await client.delete("wf2");
  assertEqual(result.deleted, "wf2"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows?id=wf2"); assertEqual(calls[0]?.init?.method, "DELETE"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("WorkflowWriteClient exposes nested write errors", async () => {
  const client = createWorkflowWriteClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested workflow write failure" } }, { status: 400 }) });
  try { await client.save({ name: "", nodes: [], edges: [] }); throw new Error("expected save to reject"); } catch (error) { assert(error instanceof WorkflowWriteClientError); assertEqual(error.name, "WorkflowClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { message: "nested workflow write failure" } }); assertEqual(error.message, "nested workflow write failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
