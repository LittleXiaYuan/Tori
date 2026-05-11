import { createSchedulerClient, SchedulerClientError } from "./scheduler";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SchedulerClient lists jobs with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSchedulerClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ jobs: [{ id: "job_1", name: "daily", interval: 60000000000, prompt: "复盘" }], count: 1 }); } });
  const result = await client.jobs();
  assertEqual(result.jobs[0]?.id, "job_1"); assertEqual(result.count, 1); assertEqual(calls[0]?.url, "http://localhost:9090/v1/scheduler/jobs"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SchedulerClient adds prompt jobs with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSchedulerClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ id: "job_2", name: "hourly", tenant_id: "default", interval: 3600000000000, prompt: "检查任务" }, { status: 201 }); } });
  const job = await client.add({ name: "hourly", prompt: "检查任务", interval: "1h" });
  assertEqual(job.id, "job_2"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/scheduler/add"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "hourly", prompt: "检查任务", interval: "1h" });
});

test("SchedulerClient removes jobs by body id", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSchedulerClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "removed" }); } });
  const removed = await client.remove("job_1");
  assertEqual(removed.status, "removed"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/scheduler/remove"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "job_1" });
});

test("SchedulerClient throws SchedulerClientError with parsed and text bodies", async () => {
  const jsonClient = createSchedulerClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "invalid interval (min 1m)" }, { status: 400 }) });
  try { await jsonClient.add({ name: "bad", prompt: "x", interval: "1s" }); throw new Error("expected add to reject"); } catch (error) { assert(error instanceof SchedulerClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "invalid interval (min 1m)" }); assertEqual(error.message, "invalid interval (min 1m)"); }
  const textClient = createSchedulerClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.jobs(); throw new Error("expected jobs to reject"); } catch (error) { assert(error instanceof SchedulerClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
