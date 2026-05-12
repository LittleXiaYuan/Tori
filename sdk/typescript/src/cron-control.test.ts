import { createCronControlClient, CronControlClientError } from "./cron-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CronControlClient adds jobs with bearer token and JSON body", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createCronControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ job: { id: "job-2", name: "nightly", schedule: { type: "cron", cron_expr: "0 2 * * *", timezone: "Asia/Shanghai" }, payload: { kind: "systemEvent", data: { event: "nightly" } }, enabled: true, created_at: "2026-05-11T00:00:00Z", run_count: 0 } }); } });
  const result = await client.add({ name: "nightly", schedule: { type: "cron", cron_expr: "0 2 * * *", timezone: "Asia/Shanghai" }, payload: { kind: "systemEvent", data: { event: "nightly" } } });
  assertEqual(result.job.id, "job-2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cron/add");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), "application/json");
  assertDeepEqual(calls[0]?.body, { name: "nightly", schedule: { type: "cron", cron_expr: "0 2 * * *", timezone: "Asia/Shanghai" }, payload: { kind: "systemEvent", data: { event: "nightly" } } });
});

test("CronControlClient removes and runs jobs with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCronControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("remove")) return jsonResponse({ deleted: "job-1" }); return jsonResponse({ run: { job_id: "job-1", run_id: "run-1", started_at: "2026-05-11T00:00:00Z", ended_at: "2026-05-11T00:00:01Z", status: "success", output: "ok" } }); } });
  const removed = await client.remove("job-1");
  const run = await client.run("job-1");
  assertEqual(removed.deleted, "job-1");
  assertEqual(run.run.status, "success");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cron/remove?id=job-1");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cron/run?id=job-1");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CronControlClient exposes nested control errors", async () => {
  const client = createCronControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "CRON_CONTROL", message: "cron expression is required" } }, { status: 400 }) });
  try { await client.add({ name: "bad", schedule: { type: "cron", cron_expr: "", timezone: "Asia/Shanghai" }, payload: { kind: "systemEvent", data: {} } }); throw new Error("expected add to reject"); } catch (error) { assert(error instanceof CronControlClientError); assertEqual(error.name, "CronClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "CRON_CONTROL", message: "cron expression is required" } }); assertEqual(error.message, "cron expression is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
