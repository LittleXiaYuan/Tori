import { createOrchestratorClient, OrchestratorClientError } from "./orchestrator";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("OrchestratorClient reads status and sessions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createOrchestratorClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/sessions")) return jsonResponse({ sessions: [{ session_id: "s1", adapter: "cursor", task_id: "t1", started_at: "2026-05-12T00:00:00Z" }] }); return jsonResponse({ running: true, adapters: ["cursor"], active_sessions: 1, event_count: 2, policy: { allow_auto_launch: true } }); } });
  const status = await client.status(); const sessions = await client.sessions();
  assertEqual(status.running, true); assertEqual(sessions.sessions[0]?.adapter, "cursor"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/status"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/orchestrator/sessions"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("OrchestratorClient toggles daemon and detects IDEs with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createOrchestratorClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ status: "started" }); return jsonResponse({ ides: [{ name: "Cursor", available: true }] }); } });
  const toggled = await client.toggle("start"); const detected = await client.detectIDEs();
  assertEqual(toggled.status, "started"); assertEqual(detected.ides[0]?.name, "Cursor"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/toggle"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { action: "start" }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("OrchestratorClient reads events and task timeline", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const event = { id: "e1", type: "task_assigned", task_id: "t1", message: "assigned", timestamp: "2026-05-12T00:00:00Z" };
  const client = createOrchestratorClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/events/task")) return jsonResponse({ task_id: "t1", events: [event] }); return jsonResponse({ events: [event], total: 1 }); } });
  const events = await client.events(25); const timeline = await client.taskTimeline("t1");
  assertEqual(events.total, 1); assertEqual(timeline.task_id, "t1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/events?limit=25"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/orchestrator/events/task?task_id=t1");
});

test("OrchestratorClient manages policy and custom adapters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createOrchestratorClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/policy") && init?.method === "GET") return jsonResponse({ allow_auto_launch: false, require_approval: true }); if (String(url).endsWith("/policy")) return jsonResponse({ status: "updated", policy: { allow_auto_launch: true } }); return jsonResponse({ status: "registered", name: "custom", available: true }, { status: 201 }); } });
  const policy = await client.policy(); const updated = await client.updatePolicy({ allow_auto_launch: true }); const adapter = await client.addAdapter({ adapter_name: "custom", binary: "worker.exe", mcp_config_path: "mcp.json", lifecycle: "persistent" });
  assertEqual(policy.require_approval, true); assertEqual(updated.policy.allow_auto_launch, true); assertEqual(adapter.name, "custom"); assertEqual(calls[1]?.init?.method, "PUT"); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { adapter_name: "custom", binary: "worker.exe", mcp_config_path: "mcp.json", lifecycle: "persistent" });
});

test("OrchestratorClient throws OrchestratorClientError with parsed and text bodies", async () => {
  const jsonClient = createOrchestratorClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "task_id required" }, { status: 400 }) });
  try { await jsonClient.taskTimeline(""); throw new Error("expected taskTimeline to reject"); } catch (error) { assert(error instanceof OrchestratorClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "task_id required" }); assertEqual(error.message, "task_id required"); }
  const nestedClient = createOrchestratorClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "orchestrator task id is required" } }, { status: 400 }) });
  try { await nestedClient.taskTimeline(""); throw new Error("expected taskTimeline to reject"); } catch (error) { assert(error instanceof OrchestratorClientError); assertEqual(error.status, 400); assertEqual(error.message, "orchestrator task id is required"); }
  const textClient = createOrchestratorClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.status(); throw new Error("expected status to reject"); } catch (error) { assert(error instanceof OrchestratorClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
