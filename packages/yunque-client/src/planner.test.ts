import { createPlannerClient, PlannerClientError } from "./planner";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PlannerClient lists checkpoints through planner facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ checkpoints: [{ plan_id: "plan-1", status: "failed" }], count: 1 }); } });

  const result = await client.listCheckpoints({ limit: 5, plan_id: "plan-1", include_snapshot: true });

  assertEqual(result.checkpoints[0]?.plan_id, "plan-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints?limit=5&plan_id=plan-1&include_snapshot=true");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("PlannerClient resumes direct planner jobs and reads execution state", async () => {
  const calls: string[] = [];
  const client = createPlannerClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); if (String(url).includes("execution-state")) return jsonResponse({ plan_id: "plan-1", status: "failed", next_action: "retry_failed" }); return jsonResponse({ status: "accepted", plan_id: "plan-1", job_id: "resume-1" }); } });

  const accepted = await client.resumeCheckpointPlan({ plan_id: "plan-1", action: "retry_failed", async: true });
  const state = await client.getExecutionState({ plan_id: "plan-1", action: "retry_failed" });

  assertEqual(accepted.job_id, "resume-1");
  assertEqual(state.next_action, "retry_failed");
  assertEqual(calls[0], "http://localhost:9090/v1/planner/checkpoints/resume-plan");
  assertEqual(calls[1], "http://localhost:9090/v1/planner/execution-state?plan_id=plan-1&action=retry_failed");
});

test("PlannerClient exposes planner-named errors", async () => {
  const client = createPlannerClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "checkpoint not found" } }, { status: 404 }) });

  try {
    await client.listCheckpoints({ plan_id: "missing" });
    throw new Error("expected listCheckpoints to reject");
  } catch (error) {
    assert(error instanceof PlannerClientError);
    assertEqual(error.status, 404);
    assertEqual(error.message, "checkpoint not found");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
