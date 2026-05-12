import { createCostAlertsClient, CostAlertsClientError } from "./cost-alerts";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CostAlertsClient lists alerts with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostAlertsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ alerts: [{ type: "daily" }], today_cost: 0.42 }); } });
  const result = await client.list();
  assertEqual(result.alerts?.length, 1);
  assertEqual(result.today_cost, 0.42);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/alerts");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CostAlertsClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostAlertsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ alerts: [] }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CostAlertsClient exposes nested alert errors", async () => {
  const client = createCostAlertsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "COST_ALERTS", message: "cost alerts denied" } }, { status: 403 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof CostAlertsClientError); assertEqual(error.name, "CostClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "COST_ALERTS", message: "cost alerts denied" } }); assertEqual(error.message, "cost alerts denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
