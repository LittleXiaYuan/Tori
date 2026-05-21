import { createOrchestratorReadClient, OrchestratorReadClientError } from "./orchestrator-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("OrchestratorReadClient reads status sessions and policy with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createOrchestratorReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/sessions")) return jsonResponse({ sessions: [{ session_id: "s1", adapter: "cursor", task_id: "t1", started_at: "2026-05-12T00:00:00Z" }] }); if (String(url).endsWith("/policy")) return jsonResponse({ allow_auto_launch: true }); return jsonResponse({ running: true, adapters: ["cursor"], active_sessions: 1, event_count: 2, policy: { allow_auto_launch: true } }); } });
  assertEqual((await client.status()).running, true);
  assertEqual((await client.sessions()).sessions[0]?.adapter, "cursor");
  assertEqual((await client.policy()).allow_auto_launch, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/orchestrator/sessions");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("OrchestratorReadClient detects IDEs and reads events with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const event = { id: "e1", type: "task_assigned", task_id: "t1", message: "assigned", timestamp: "2026-05-12T00:00:00Z" };
  const client = createOrchestratorReadClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("detect")) return jsonResponse({ ides: [{ name: "Cursor", available: true }] }); if (String(url).includes("/events/task")) return jsonResponse({ task_id: "t1", events: [event] }); return jsonResponse({ events: [event], total: 1 }); } });
  assertEqual((await client.detectIDEs()).ides[0]?.name, "Cursor");
  assertEqual((await client.events(25)).total, 1);
  assertEqual((await client.taskTimeline("t1")).task_id, "t1");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/orchestrator/events?limit=25");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/orchestrator/events/task?task_id=t1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("OrchestratorReadClient exposes nested read errors", async () => {
  const client = createOrchestratorReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "ORCH_READ", message: "task_id required" } }, { status: 400 }) });
  try { await client.taskTimeline(""); throw new Error("expected taskTimeline to reject"); } catch (error) { assert(error instanceof OrchestratorReadClientError); assertEqual(error.name, "OrchestratorClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "ORCH_READ", message: "task_id required" } }); assertEqual(error.message, "task_id required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
