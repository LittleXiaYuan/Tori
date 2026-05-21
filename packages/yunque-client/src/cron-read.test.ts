import { createCronReadClient, CronReadClientError } from "./cron-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CronReadClient lists jobs with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCronReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ jobs: [{ id: "job-1", name: "daily", schedule: { type: "every", every_ms: 60000 }, payload: { kind: "agentTurn", message: "ping" }, enabled: true, created_at: "2026-05-11T00:00:00Z", run_count: 0 }] }); } });
  const result = await client.list();
  assertEqual(result.jobs[0]?.name, "daily");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cron/list");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CronReadClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCronReadClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ jobs: [] }); } });
  assertEqual((await client.list()).jobs.length, 0);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CronReadClient exposes nested read errors", async () => {
  const client = createCronReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "CRON_READ", message: "list failed" } }, { status: 500 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof CronReadClientError); assertEqual(error.name, "CronClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "CRON_READ", message: "list failed" } }); assertEqual(error.message, "list failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
