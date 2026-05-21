import { createPlannerReadClient, PlannerReadClientError } from "./planner-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PlannerReadClient lists checkpoints with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerReadClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ checkpoints: [{ plan_id: "plan-1", status: "failed" }], count: 1 }); } });
  const result = await client.listCheckpoints({ limit: 5, plan_id: "plan-1", include_snapshot: true });
  assertEqual(result.checkpoints[0]?.plan_id, "plan-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints?limit=5&plan_id=plan-1&include_snapshot=true"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("PlannerReadClient reads resume job and execution state with custom API-key header", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerReadClient({ baseUrl: "http://localhost:9090", headers: { "X-API-Key": "dev-key" }, fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("execution-state")) return jsonResponse({ plan_id: "plan-1", status: "failed", next_action: "retry_failed" }); return jsonResponse({ job: { status: "running", id: "resume-1", plan_id: "plan-1", events: [{ type: "step", message: "retry" }] } }); } });
  const job = await client.getResumePlanJob({ id: "resume-1" });
  const state = await client.getExecutionState({ plan_id: "plan-1", action: "retry_failed" });
  assertEqual(job.job?.id, "resume-1"); assertEqual(state.next_action, "retry_failed"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints/resume-plan/jobs?id=resume-1"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/planner/execution-state?plan_id=plan-1&action=retry_failed"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "dev-key");
});

test("PlannerReadClient exposes nested read errors", async () => {
  const client = createPlannerReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "checkpoint not found" } }, { status: 404 }) });
  try { await client.listCheckpoints({ plan_id: "missing" }); throw new Error("expected listCheckpoints to reject"); } catch (error) { assert(error instanceof PlannerReadClientError); assertEqual(error.name, "PlannerRecoveryError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "checkpoint not found" } }); assertEqual(error.message, "checkpoint not found"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
