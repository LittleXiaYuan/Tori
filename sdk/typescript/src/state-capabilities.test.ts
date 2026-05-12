import { createStateCapabilitiesClient, StateCapabilitiesClientError } from "./state-capabilities";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("StateCapabilitiesClient reads current capability snapshot with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createStateCapabilitiesClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ goals: [], resources: [], capabilities: { total_skills: 23, dynamic_skills: ["doc_reader"], unresolved_gaps: 2, recent_gaps: ["missing_parser"] } }); } });
  const capabilities = await client.get();
  assertEqual(capabilities.total_skills, 23);
  assertEqual(capabilities.dynamic_skills?.[0], "doc_reader");
  assertEqual(capabilities.unresolved_gaps, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/state");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("StateCapabilitiesClient returns an empty object when snapshot has no capabilities", async () => {
  const client = createStateCapabilitiesClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (_url, init) => { assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123"); return jsonResponse({ goals: [], resources: [] }); } });
  const capabilities = await client.get();
  assertDeepEqual(capabilities, {});
});

test("StateCapabilitiesClient exposes capabilities nested gateway errors", async () => {
  const client = createStateCapabilitiesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "NOT_READY", message: "nested state capabilities failure" } }, { status: 503 }) });
  try { await client.get(); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof StateCapabilitiesClientError); assertEqual(error.name, "StateClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "NOT_READY", message: "nested state capabilities failure" } }); assertEqual(error.message, "nested state capabilities failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
