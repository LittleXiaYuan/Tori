import { createSystemProbesClient, SystemProbesClientError } from "./system-probes";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SystemProbesClient reads health livez and version with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSystemProbesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const text = String(url); if (text.endsWith("/healthz")) return jsonResponse({ status: "ok", version: "dev" }); if (text.endsWith("/livez")) return jsonResponse({ status: "ok", uptime_sec: 3 }); return jsonResponse({ version: "0.1.0", git_commit: "abc" }); } });
  assertEqual((await client.health()).status, "ok");
  assertEqual((await client.livez()).uptime_sec, 3);
  assertEqual((await client.version()).git_commit, "abc");
  assertDeepEqual(calls.map((call) => call.url), ["http://localhost:9090/healthz", "http://localhost:9090/livez", "http://localhost:9090/v1/version"]);
  assertEqual(new Headers(calls[2]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SystemProbesClient returns readiness and cognitive 503 bodies as probe results", async () => {
  const client = createSystemProbesClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { if (String(url).endsWith("/readyz")) return jsonResponse({ status: "not_ready", checks: { llm: { status: "down" } } }, { status: 503 }); return jsonResponse({ status: "unhealthy", summary: { down: 1 } }, { status: 503 }); } });
  assertEqual((await client.readyz()).checks?.llm?.status, "down");
  assertEqual((await client.cognitiveHealth()).summary?.down, 1);
});

test("SystemProbesClient exposes nested probe errors", async () => {
  const client = createSystemProbesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SYSTEM_PROBES", message: "probe failed" } }, { status: 500 }) });
  try { await client.version(); throw new Error("expected version to reject"); } catch (error) { assert(error instanceof SystemProbesClientError); assertEqual(error.name, "SystemClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "SYSTEM_PROBES", message: "probe failed" } }); assertEqual(error.message, "probe failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
