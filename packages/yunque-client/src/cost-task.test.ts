import { createCostTaskClient, CostTaskClientError } from "./cost-task";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CostTaskClient reads task and timeline with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostTaskClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return String(url).includes("timeline") ? jsonResponse([{ cost_usd: 0.03 }]) : jsonResponse({ total_cost: 0.08 }); } });
  assertEqual((await client.get("task/id") as { total_cost?: number }).total_cost, 0.08);
  assertEqual((await client.timeline("task/id") as Array<{ cost_usd?: number }>)[0]?.cost_usd, 0.03);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/task?id=task%2Fid");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cost/task/timeline?id=task%2Fid");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CostTaskClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostTaskClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ total_cost: 0 }); } });
  await client.get("task");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CostTaskClient exposes nested task errors", async () => {
  const client = createCostTaskClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "COST_TASK", message: "task cost unavailable" } }, { status: 404 }) });
  try { await client.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof CostTaskClientError); assertEqual(error.name, "CostClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { code: "COST_TASK", message: "task cost unavailable" } }); assertEqual(error.message, "task cost unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
