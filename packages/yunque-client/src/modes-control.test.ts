import { createModesControlClient, ModesControlClientError } from "./modes-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ModesControlClient sets mode with bearer token", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createModesControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ success: true, current_mode: "coder" }); } });
  const result = await client.set({ mode: "coder", tenant_id: "tenant-1", session_id: "session-1" });
  assertEqual(result.success, true);
  assertEqual(result.current_mode, "coder");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/mode");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), "application/json");
  assertDeepEqual(calls[0]?.body, { mode: "coder", tenant_id: "tenant-1", session_id: "session-1" });
});

test("ModesControlClient sets mode with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createModesControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ success: true, current_mode: "operator", modes: [{ mode: "operator", active: true }] }); } });
  const result = await client.set({ mode: "operator" });
  assertEqual(result.current_mode, "operator");
  assertEqual(result.modes?.[0]?.active, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/mode");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ModesControlClient exposes nested control errors", async () => {
  const client = createModesControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "MODES_CONTROL", message: "switch failed" } }, { status: 409 }) });
  try { await client.set({ mode: "coder" }); throw new Error("expected set to reject"); } catch (error) { assert(error instanceof ModesControlClientError); assertEqual(error.name, "ModesClientError"); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: { code: "MODES_CONTROL", message: "switch failed" } }); assertEqual(error.message, "switch failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
