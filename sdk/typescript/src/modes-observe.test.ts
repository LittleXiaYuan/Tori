import { createModesObserveClient, ModesObserveClientError } from "./modes-observe";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ModesObserveClient lists modes with bearer token and scope", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createModesObserveClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ modes: [{ mode: "coder", active: true }], total: 1 }); } });
  const result = await client.list({ tenant_id: "tenant-1", session_id: "session-1" });
  assertEqual(result.total, 1);
  assertEqual(result.modes[0]?.mode, "coder");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/modes?tenant_id=tenant-1&session_id=session-1");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ModesObserveClient reads current mode with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createModesObserveClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ mode: "researcher", name: "Researcher" }); } });
  assertEqual((await client.current({ session_id: "s2" })).mode, "researcher");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/mode/current?session_id=s2");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ModesObserveClient exposes nested observe errors", async () => {
  const client = createModesObserveClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "MODES_OBSERVE", message: "observe failed" } }, { status: 500 }) });
  try { await client.current(); throw new Error("expected current to reject"); } catch (error) { assert(error instanceof ModesObserveClientError); assertEqual(error.name, "ModesClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "MODES_OBSERVE", message: "observe failed" } }); assertEqual(error.message, "observe failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
