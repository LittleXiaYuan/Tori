import { createCostBudgetClient, CostBudgetClientError } from "./cost-budget";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CostBudgetClient reads summary and alerts with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostBudgetClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/alerts")) return jsonResponse({ alerts: [{ type: "daily" }], today_cost: 0.4 }); return jsonResponse({ today_cost: 0.3, month_cost: 2 }); } });
  assertEqual((await client.summary()).today_cost, 0.3);
  assertEqual((await client.alerts()).alerts?.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/summary");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cost/alerts");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CostBudgetClient sets budget with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostBudgetClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true }); } });
  assertEqual((await client.setBudget({ daily_limit_usd: 1, monthly_limit_usd: 10 })).ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/budget");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { daily_limit_usd: 1, monthly_limit_usd: 10 });
});

test("CostBudgetClient exposes nested budget errors", async () => {
  const client = createCostBudgetClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BUDGET", message: "budget failed" } }, { status: 429 }) });
  try { await client.setBudget({ daily_limit_usd: 1 }); throw new Error("expected setBudget to reject"); } catch (error) { assert(error instanceof CostBudgetClientError); assertEqual(error.name, "CostClientError"); assertEqual(error.status, 429); assertDeepEqual(error.body, { error: { code: "BUDGET", message: "budget failed" } }); assertEqual(error.message, "budget failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
