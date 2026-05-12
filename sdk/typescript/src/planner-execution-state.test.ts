import { createPlannerExecutionStateClient, PlannerExecutionStateClientError } from "./planner-execution-state";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PlannerExecutionStateClient reads execution state with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerExecutionStateClient({ baseUrl: "http://localhost:9090", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ plan_id: "plan-1", status: "failed", next_action: "retry_failed" }); } });
  const result = await client.get({ plan_id: "plan-1", action: "retry_failed" });
  assertEqual(result.next_action, "retry_failed"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/execution-state?plan_id=plan-1&action=retry_failed"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PlannerExecutionStateClient reads resume jobs with API key headers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerExecutionStateClient({ baseUrl: "http://localhost:9090/", headers: { "X-API-Key": "dev-key" }, fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ job: { id: "job-1", plan_id: "plan-1", status: "running" } }); } });
  const result = await client.job({ id: "job-1" });
  assertEqual(result.job?.id, "job-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints/resume-plan/jobs?id=job-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "dev-key");
});

test("PlannerExecutionStateClient exposes nested execution errors", async () => {
  const client = createPlannerExecutionStateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested execution failure" } }, { status: 404 }) });
  try { await client.get({ plan_id: "missing" }); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof PlannerExecutionStateClientError); assertEqual(error.name, "PlannerRecoveryError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "nested execution failure" } }); assertEqual(error.message, "nested execution failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
