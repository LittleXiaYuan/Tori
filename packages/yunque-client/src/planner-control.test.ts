import { createPlannerControlClient, PlannerControlClientError } from "./planner-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PlannerControlClient recovers checkpoints with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerControlClient({ baseUrl: "http://localhost:9090", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ recovered: true, plan_id: "plan-1", action: "retry_failed" }); } });
  const result = await client.recoverCheckpoint({ plan_id: "plan-1", action: "retry_failed" });
  assertEqual(result.plan_id, "plan-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints/recover"); assertEqual(calls[0]?.init?.method, "POST"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { plan_id: "plan-1", action: "retry_failed" }); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PlannerControlClient resumes checkpoint tasks and plans with custom API-key header", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerControlClient({ baseUrl: "http://localhost:9090", headers: { "X-API-Key": "dev-key" }, fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "accepted", plan_id: "plan-1", job_id: "resume-1" }); } });
  const task = await client.resumeCheckpointTask({ plan_id: "plan-1", action: "continue", run: true });
  const plan = await client.resumeCheckpointPlan({ plan_id: "plan-1", action: "retry_failed", async: true });
  assertEqual(task.status, "accepted"); assertEqual(plan.job_id, "resume-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints/resume"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/planner/checkpoints/resume-plan"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { plan_id: "plan-1", action: "continue", run: true }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { plan_id: "plan-1", action: "retry_failed", async: true }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "dev-key");
});

test("PlannerControlClient exposes nested control errors", async () => {
  const client = createPlannerControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "resume plan failed" } }, { status: 409 }) });
  try { await client.resumeCheckpointPlan({ plan_id: "plan-1", action: "retry_failed" }); throw new Error("expected resumeCheckpointPlan to reject"); } catch (error) { assert(error instanceof PlannerControlClientError); assertEqual(error.name, "PlannerRecoveryError"); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: { message: "resume plan failed" } }); assertEqual(error.message, "resume plan failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
