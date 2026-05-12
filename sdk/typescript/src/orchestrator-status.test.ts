import { createOrchestratorStatusClient, OrchestratorStatusClientError } from "./orchestrator-status";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("OrchestratorStatusClient reads status sessions and policy with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createOrchestratorStatusClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/sessions")) return jsonResponse({ sessions: [{ session_id: "s1", adapter: "cursor", task_id: "t1", started_at: "2026-05-12T00:00:00Z" }] }); if (String(url).endsWith("/policy")) return jsonResponse({ allow_auto_launch: true }); return jsonResponse({ running: true, adapters: ["cursor"], active_sessions: 1, event_count: 2, policy: { allow_auto_launch: true } }); } });
  assertEqual((await client.status()).running, true);
  assertEqual((await client.sessions()).sessions[0]?.adapter, "cursor");
  assertEqual((await client.policy()).allow_auto_launch, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/orchestrator/sessions");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/orchestrator/policy");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("OrchestratorStatusClient supports API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createOrchestratorStatusClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ running: false, adapters: [], active_sessions: 0 }); } });
  assertEqual((await client.status()).running, false);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/status");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("OrchestratorStatusClient exposes orchestrator-status nested gateway errors", async () => {
  const client = createOrchestratorStatusClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "ORCH_STATUS", message: "nested orchestrator status failure" } }, { status: 503 }) });
  try { await client.status(); throw new Error("expected status to reject"); } catch (error) { assert(error instanceof OrchestratorStatusClientError); assertEqual(error.name, "OrchestratorClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "ORCH_STATUS", message: "nested orchestrator status failure" } }); assertEqual(error.message, "nested orchestrator status failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
