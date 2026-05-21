import { createCostObserveClient, CostObserveClientError } from "./cost-observe";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CostObserveClient reads task, timeline and breakdown with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostObserveClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.includes("/task/timeline")) return jsonResponse([{ cost_usd: 0.01 }]); if (path.includes("/task?")) return jsonResponse({ total_cost: 0.02 }); return jsonResponse({ by_provider: { openai: 0.1 } }); } });
  assertEqual((await client.task("task/id") as { total_cost?: number }).total_cost, 0.02);
  assertEqual((await client.taskTimeline("task/id") as Array<{ cost_usd?: number }>)[0]?.cost_usd, 0.01);
  assertEqual((await client.breakdown()).by_provider?.openai, 0.1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/task?id=task%2Fid");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cost/task/timeline?id=task%2Fid");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cost/breakdown");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CostObserveClient reads filtered history with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostObserveClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ records: [], page: 2 }); } });
  assertEqual((await client.history({ page: 2, limit: 25, model: "gpt-test", provider_id: "p1" }) as { page?: number }).page, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/history?page=2&limit=25&model=gpt-test&provider_id=p1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CostObserveClient exposes nested observe errors", async () => {
  const client = createCostObserveClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "COST_OBSERVE", message: "observe failed" } }, { status: 400 }) });
  try { await client.task("task"); throw new Error("expected task to reject"); } catch (error) { assert(error instanceof CostObserveClientError); assertEqual(error.name, "CostClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "COST_OBSERVE", message: "observe failed" } }); assertEqual(error.message, "observe failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
