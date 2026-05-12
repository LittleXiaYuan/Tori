import { createCostHistoryClient, CostHistoryClientError } from "./cost-history";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CostHistoryClient lists filtered history with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostHistoryClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ records: [{ id: "r1" }], page: 2 }); } });
  const result = await client.list({ page: 2, limit: 25, task_id: "task/id", model: "gpt-test", channel: "chat", runner_type: "agent", provider_id: "p1" }) as { page?: number };
  assertEqual(result.page, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/history?page=2&limit=25&task_id=task%2Fid&model=gpt-test&channel=chat&runner_type=agent&provider_id=p1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CostHistoryClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostHistoryClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ records: [] }); } });
  await client.list();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/history");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CostHistoryClient exposes nested history errors", async () => {
  const client = createCostHistoryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "COST_HISTORY", message: "history query failed" } }, { status: 400 }) });
  try { await client.list({ page: 1 }); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof CostHistoryClientError); assertEqual(error.name, "CostClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "COST_HISTORY", message: "history query failed" } }); assertEqual(error.message, "history query failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
