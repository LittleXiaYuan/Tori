import { createSystemOpsClient, SystemOpsClientError } from "./system-ops";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SystemOpsClient reads info stats metrics cache and modules with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSystemOpsClient({ baseUrl: "http://localhost:9090/", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); const text = String(url); if (text.endsWith("/system/info")) return jsonResponse({ breaker: { state: "closed" } }); if (text.endsWith("/system/stats")) return jsonResponse({ requests_total: 12 }); if (text.endsWith("/metrics")) return jsonResponse({ requests_total: 12 }); if (text.endsWith("/cache/stats")) return jsonResponse({ llm_response_cache: { entries: 2 } }); return jsonResponse({ modules: [{ id: "chat" }], profile: "dev" }); } });
  assertEqual((await client.systemInfo()).breaker?.state, "closed");
  assertEqual((await client.systemStats()).requests_total, 12);
  assertEqual((await client.metrics()).requests_total, 12);
  assertEqual(((await client.cacheStats()).llm_response_cache as { entries?: number }).entries, 2);
  assertEqual((await client.modules()).modules.length, 1);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SystemOpsClient reads prometheus metrics and SBOM with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSystemOpsClient({ baseUrl: "http://localhost:9090", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/metrics/prometheus")) return new Response("yunque_requests_total 12\n", { status: 200, headers: { "Content-Type": "text/plain" } }); return jsonResponse({ bomFormat: "CycloneDX", components: [{ name: "yunque-agent" }] }); } });
  assert((await client.metricsPrometheus()).includes("yunque_requests_total"));
  assertEqual((await client.sbom()).bomFormat, "CycloneDX");
  assertDeepEqual(calls.map((call) => call.url), ["http://localhost:9090/v1/metrics/prometheus", "http://localhost:9090/sbom"]);
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SystemOpsClient exposes nested ops errors", async () => {
  const client = createSystemOpsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SYSTEM_OPS", message: "ops failed" } }, { status: 500 }) });
  try { await client.metrics(); throw new Error("expected metrics to reject"); } catch (error) { assert(error instanceof SystemOpsClientError); assertEqual(error.name, "SystemClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "SYSTEM_OPS", message: "ops failed" } }); assertEqual(error.message, "ops failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
