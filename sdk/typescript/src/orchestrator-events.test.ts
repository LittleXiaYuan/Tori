import { createOrchestratorEventsClient, OrchestratorEventsClientError } from "./orchestrator-events";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
const event = { id: "e1", type: "task_assigned", task_id: "t1", message: "assigned", timestamp: "2026-05-12T00:00:00Z" };

test("OrchestratorEventsClient reads recent events with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createOrchestratorEventsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ events: [event], total: 1 }); } });
  assertEqual((await client.events(25)).total, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/events?limit=25");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("OrchestratorEventsClient reads task timeline with API key and encodes task id", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createOrchestratorEventsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ task_id: "t 1/二", events: [event] }); } });
  assertEqual((await client.taskTimeline("t 1/二")).task_id, "t 1/二");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/events/task?task_id=t+1%2F%E4%BA%8C");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("OrchestratorEventsClient exposes orchestrator-events nested gateway errors", async () => {
  const client = createOrchestratorEventsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "ORCH_EVENTS", message: "nested orchestrator events failure" } }, { status: 400 }) });
  try { await client.taskTimeline(""); throw new Error("expected taskTimeline to reject"); } catch (error) { assert(error instanceof OrchestratorEventsClientError); assertEqual(error.name, "OrchestratorClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "ORCH_EVENTS", message: "nested orchestrator events failure" } }); assertEqual(error.message, "nested orchestrator events failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
