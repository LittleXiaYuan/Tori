import { createSchedulerReadClient, SchedulerReadClientError } from "./scheduler-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SchedulerReadClient lists jobs with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSchedulerReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ jobs: [{ id: "job_1", name: "daily", interval: 60000000000, prompt: "复盘" }], count: 1 }); } });
  const result = await client.jobs();
  assertEqual(result.jobs[0]?.id, "job_1");
  assertEqual(result.count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/scheduler/jobs");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SchedulerReadClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSchedulerReadClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ jobs: [], count: 0 }); } });
  assertEqual((await client.jobs()).count, 0);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SchedulerReadClient exposes nested read errors", async () => {
  const client = createSchedulerReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SCHEDULER_READ", message: "read failed" } }, { status: 500 }) });
  try { await client.jobs(); throw new Error("expected jobs to reject"); } catch (error) { assert(error instanceof SchedulerReadClientError); assertEqual(error.name, "SchedulerClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "SCHEDULER_READ", message: "read failed" } }); assertEqual(error.message, "read failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
