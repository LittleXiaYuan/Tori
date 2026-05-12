import { createSetupDetectClient, SetupDetectClientError } from "./setup-detect";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SetupDetectClient detects environment and health with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupDetectClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/detect")) return jsonResponse({ has_docker: true, has_gpu: false, has_ollama: true }); return jsonResponse({ providers: [{ id: "ollama", available: true }], has_docker: true }); } });
  const detected = await client.detect();
  const health = await client.health();
  assertEqual(detected.has_docker, true);
  assertEqual(detected.has_ollama, true);
  assertEqual(health.providers?.[0]?.id, "ollama");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/detect");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/setup/health");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SetupDetectClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupDetectClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ has_docker: false, providers: [] }); } });
  await client.health();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/health");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SetupDetectClient exposes nested setup errors", async () => {
  const client = createSetupDetectClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested setup detect failure" } }, { status: 400 }) });
  try { await client.detect(); throw new Error("expected detect to reject"); } catch (error) { assert(error instanceof SetupDetectClientError); assertEqual(error.name, "SetupClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested setup detect failure" } }); assertEqual(error.message, "nested setup detect failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
