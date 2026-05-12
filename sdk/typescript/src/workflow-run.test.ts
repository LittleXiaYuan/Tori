import { createWorkflowRunClient, WorkflowRunClientError } from "./workflow-run";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("WorkflowRunClient runs workflow definitions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const instance = { id: "inst1", definition_id: "wf1", status: "pending" };
  const client = createWorkflowRunClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "accepted", instance_id: "inst1", instance }, { status: 202 }); } });
  const result = await client.runDefinition("wf1", { topic: "sdk" });
  assertEqual(result.instance_id, "inst1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/run"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { definition_id: "wf1", variables: { topic: "sdk" } }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("WorkflowRunClient cancels workflow instances with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createWorkflowRunClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "cancelling", instance_id: "inst1" }); } });
  const result = await client.cancel("inst1");
  assertEqual(result.status, "cancelling"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/workflows/cancel"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { instance_id: "inst1" }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("WorkflowRunClient exposes nested run errors", async () => {
  const client = createWorkflowRunClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested workflow run failure" } }, { status: 400 }) });
  try { await client.run({ definition_id: "" }); throw new Error("expected run to reject"); } catch (error) { assert(error instanceof WorkflowRunClientError); assertEqual(error.name, "WorkflowClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { message: "nested workflow run failure" } }); assertEqual(error.message, "nested workflow run failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
