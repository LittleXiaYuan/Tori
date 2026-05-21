import { createSchedulerControlClient, SchedulerControlClientError } from "./scheduler-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SchedulerControlClient adds prompt jobs with bearer token", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createSchedulerControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ id: "job_2", name: "hourly", tenant_id: "default", interval: 3600000000000, prompt: "检查任务" }, { status: 201 }); } });
  const job = await client.add({ name: "hourly", prompt: "检查任务", interval: "1h" });
  assertEqual(job.id, "job_2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/scheduler/add");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), "application/json");
  assertDeepEqual(calls[0]?.body, { name: "hourly", prompt: "检查任务", interval: "1h" });
});

test("SchedulerControlClient removes jobs with API key", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createSchedulerControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ status: "removed" }); } });
  const removed = await client.remove("job_1");
  assertEqual(removed.status, "removed");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/scheduler/remove");
  assertDeepEqual(calls[0]?.body, { id: "job_1" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SchedulerControlClient exposes nested control errors", async () => {
  const client = createSchedulerControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SCHEDULER_CONTROL", message: "interval required" } }, { status: 400 }) });
  try { await client.add({ name: "bad", prompt: "x", interval: "" }); throw new Error("expected add to reject"); } catch (error) { assert(error instanceof SchedulerControlClientError); assertEqual(error.name, "SchedulerClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "SCHEDULER_CONTROL", message: "interval required" } }); assertEqual(error.message, "interval required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
