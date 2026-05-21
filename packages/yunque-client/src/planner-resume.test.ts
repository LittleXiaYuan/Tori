import { createPlannerResumeClient, PlannerResumeClientError } from "./planner-resume";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PlannerResumeClient resumes checkpoint tasks with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerResumeClient({ baseUrl: "http://localhost:9090", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "accepted", task_id: "task-1" }); } });
  const result = await client.task({ plan_id: "plan-1", action: "continue", run: true });
  assertEqual(result.task_id, "task-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints/resume"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { plan_id: "plan-1", action: "continue", run: true }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PlannerResumeClient resumes checkpoint plans with API key headers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerResumeClient({ baseUrl: "http://localhost:9090/", headers: { "X-API-Key": "dev-key" }, fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "accepted", plan_id: "plan-2", job_id: "job-2" }, { status: 202 }); } });
  const result = await client.plan({ plan_id: "plan-2", action: "retry_failed", async: true });
  assertEqual(result.job_id, "job-2"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints/resume-plan"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { plan_id: "plan-2", action: "retry_failed", async: true }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "dev-key");
});

test("PlannerResumeClient exposes nested resume errors", async () => {
  const client = createPlannerResumeClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested resume failure" } }, { status: 409 }) });
  try { await client.plan({ plan_id: "plan-1", action: "retry_failed" }); throw new Error("expected plan to reject"); } catch (error) { assert(error instanceof PlannerResumeClientError); assertEqual(error.name, "PlannerRecoveryError"); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: { message: "nested resume failure" } }); assertEqual(error.message, "nested resume failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
