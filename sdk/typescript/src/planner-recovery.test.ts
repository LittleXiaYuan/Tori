import { createPlannerRecoveryClient, PlannerRecoveryError } from "./planner-recovery";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) {
    throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
  }
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) {
    throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
  }
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

test("PlannerRecoveryClient lists checkpoints with auth and query params", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerRecoveryClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ checkpoints: [{ plan_id: "plan-1", recoverable: true }], count: 1 });
    },
  });

  const result = await client.listCheckpoints({ limit: 5, plan_id: "plan-1", include_snapshot: true });

  assertEqual(result.checkpoints[0]?.plan_id, "plan-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints?limit=5&plan_id=plan-1&include_snapshot=true");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("PlannerRecoveryClient posts async resume-plan without importing the generated SDK bundle", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerRecoveryClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "accepted", plan_id: "plan-2", job_id: "job-2" }, { status: 202 });
    },
  });

  const result = await client.resumeCheckpointPlan({ plan_id: "plan-2", action: "continue", async: true });

  assertEqual(result.job_id, "job-2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints/resume-plan");
  assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), "application/json");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ plan_id: "plan-2", action: "continue", async: true }));
});

test("PlannerRecoveryClient throws PlannerRecoveryError with parsed body", async () => {
  const client = createPlannerRecoveryClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "checkpoint not found" }, { status: 404 }),
  });

  try {
    await client.getExecutionState({ plan_id: "missing" });
    throw new Error("expected getExecutionState to reject");
  } catch (error) {
    assert(error instanceof PlannerRecoveryError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "checkpoint not found" });
    assertEqual(error.message, "checkpoint not found");
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
