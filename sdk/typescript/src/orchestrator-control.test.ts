import { createOrchestratorControlClient, OrchestratorControlClientError } from "./orchestrator-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("OrchestratorControlClient toggles daemon with bearer token", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createOrchestratorControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ status: "started" }); } });
  const toggled = await client.toggle("start");
  assertEqual(toggled.status, "started");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/orchestrator/toggle");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(calls[0]?.body, { action: "start" });
});

test("OrchestratorControlClient updates policy and adds adapters with API key", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createOrchestratorControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); if (String(url).endsWith("/policy")) return jsonResponse({ status: "updated", policy: { allow_auto_launch: true } }); return jsonResponse({ status: "registered", name: "custom", available: true }, { status: 201 }); } });
  assertEqual((await client.updatePolicy({ allow_auto_launch: true })).policy.allow_auto_launch, true);
  assertEqual((await client.addAdapter({ adapter_name: "custom", binary: "worker.exe", mcp_config_path: "mcp.json", lifecycle: "persistent" })).name, "custom");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/orchestrator/adapters/add");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(calls[1]?.body, { adapter_name: "custom", binary: "worker.exe", mcp_config_path: "mcp.json", lifecycle: "persistent" });
});

test("OrchestratorControlClient exposes nested control errors", async () => {
  const client = createOrchestratorControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "ORCH_CONTROL", message: "invalid action" } }, { status: 400 }) });
  try { await client.toggle("start"); throw new Error("expected toggle to reject"); } catch (error) { assert(error instanceof OrchestratorControlClientError); assertEqual(error.name, "OrchestratorClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "ORCH_CONTROL", message: "invalid action" } }); assertEqual(error.message, "invalid action"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
