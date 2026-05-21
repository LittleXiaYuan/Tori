import { createCronClient, CronClientError } from "./cron";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CronClient lists jobs with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCronClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ jobs: [{ id: "job-1", name: "daily", schedule: { type: "every", every_ms: 60000 }, payload: { kind: "agentTurn", message: "ping" }, enabled: true, created_at: "2026-05-11T00:00:00Z", run_count: 0 }] }); } });
  const result = await client.list();
  assertEqual(result.jobs[0]?.name, "daily"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/cron/list"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("CronClient adds jobs with API key and JSON body", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCronClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ job: { id: "job-2", name: "nightly", schedule: { type: "cron", cron_expr: "0 2 * * *", timezone: "Asia/Shanghai" }, payload: { kind: "systemEvent", data: { event: "nightly" } }, enabled: true, created_at: "2026-05-11T00:00:00Z", run_count: 0 } }); } });
  const result = await client.add({ name: "nightly", schedule: { type: "cron", cron_expr: "0 2 * * *", timezone: "Asia/Shanghai" }, payload: { kind: "systemEvent", data: { event: "nightly" } } });
  assertEqual(result.job.id, "job-2"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/cron/add"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), "application/json"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "nightly", schedule: { type: "cron", cron_expr: "0 2 * * *", timezone: "Asia/Shanghai" }, payload: { kind: "systemEvent", data: { event: "nightly" } } });
});

test("CronClient removes and runs jobs by id query", async () => {
  const calls: string[] = [];
  const client = createCronClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); if (String(url).includes("remove")) return jsonResponse({ deleted: "job-1" }); return jsonResponse({ run: { job_id: "job-1", run_id: "run-1", started_at: "2026-05-11T00:00:00Z", ended_at: "2026-05-11T00:00:01Z", status: "success", output: "ok" } }); } });
  const removed = await client.remove("job-1"); const run = await client.run("job-1");
  assertEqual(removed.deleted, "job-1"); assertEqual(run.run.status, "success"); assertEqual(calls[0], "http://localhost:9090/v1/cron/remove?id=job-1"); assertEqual(calls[1], "http://localhost:9090/v1/cron/run?id=job-1");
});

test("CronClient throws CronClientError with parsed and text bodies", async () => {
  const jsonClient = createCronClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "job id required" }, { status: 400 }) });
  try { await jsonClient.run(""); throw new Error("expected run to reject"); } catch (error) { assert(error instanceof CronClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "job id required" }); assertEqual(error.message, "job id required"); }
  const nestedClient = createCronClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "cron expression is required" } }, { status: 400 }) });
  try { await nestedClient.add({ name: "bad", schedule: { type: "cron", cron_expr: "", timezone: "Asia/Shanghai" }, payload: { kind: "systemEvent", data: {} } }); throw new Error("expected add to reject"); } catch (error) { assert(error instanceof CronClientError); assertEqual(error.status, 400); assertEqual(error.message, "cron expression is required"); }
  const textClient = createCronClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("not found", { status: 404 }) });
  try { await textClient.remove("missing"); throw new Error("expected remove to reject"); } catch (error) { assert(error instanceof CronClientError); assertEqual(error.status, 404); assertEqual(error.body, "not found"); assertEqual(error.message, "not found"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
