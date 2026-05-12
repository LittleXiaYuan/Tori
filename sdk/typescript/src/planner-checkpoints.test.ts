import { createPlannerCheckpointsClient, PlannerCheckpointsClientError } from "./planner-checkpoints";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PlannerCheckpointsClient lists checkpoints with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerCheckpointsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ checkpoints: [{ plan_id: "plan-1", recoverable: true }], count: 1 }); } });
  const result = await client.list({ limit: 10, plan_id: "plan-1", include_snapshot: true });
  assertEqual(result.checkpoints[0]?.plan_id, "plan-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints?limit=10&plan_id=plan-1&include_snapshot=true"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PlannerCheckpointsClient supports API key headers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPlannerCheckpointsClient({ baseUrl: "http://localhost:9090", headers: { "X-API-Key": "dev-key" }, fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ checkpoints: [] }); } });
  await client.list();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/planner/checkpoints"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "dev-key");
});

test("PlannerCheckpointsClient exposes nested checkpoint errors", async () => {
  const client = createPlannerCheckpointsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested checkpoint failure" } }, { status: 500 }) });
  try { await client.list({ plan_id: "missing" }); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof PlannerCheckpointsClientError); assertEqual(error.name, "PlannerRecoveryError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { message: "nested checkpoint failure" } }); assertEqual(error.message, "nested checkpoint failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
