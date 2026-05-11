import { createSystemClient, SystemClientError } from "./system";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SystemClient reads public health probes without generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSystemClient({ baseUrl: "http://localhost:9090/", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/healthz")) return jsonResponse({ status: "ok", version: "dev", breaker_state: "closed", uptime_sec: 3 }); return jsonResponse({ status: "ok", uptime_sec: 3 }); } });
  const health = await client.health(); const live = await client.livez();
  assertEqual(health.status, "ok"); assertEqual(health.breaker_state, "closed"); assertEqual(live.uptime_sec, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/healthz"); assertEqual(calls[1]?.url, "http://localhost:9090/livez");
});

test("SystemClient returns readiness and cognitive 503 bodies as probe results", async () => {
  const client = createSystemClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { if (String(url).endsWith("/readyz")) return jsonResponse({ status: "not_ready", checks: { llm: { status: "down" } } }, { status: 503 }); return jsonResponse({ status: "unhealthy", summary: { ok: 1, down: 1 }, resources: { goroutines: 7 } }, { status: 503 }); } });
  const ready = await client.readyz(); const cognitive = await client.cognitiveHealth();
  assertEqual(ready.status, "not_ready"); assertEqual(ready.checks?.llm?.status, "down"); assertEqual(cognitive.status, "unhealthy"); assertEqual(cognitive.summary?.down, 1);
});

test("SystemClient reads version and authenticated system info with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSystemClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/version")) return jsonResponse({ version: "0.1.0", git_commit: "abc" }); return jsonResponse({ system: { os: "windows" }, breaker: { state: "closed", failures: 0 } }); } });
  const version = await client.version(); const info = await client.systemInfo();
  assertEqual(version.git_commit, "abc"); assertEqual(info.breaker?.state, "closed"); assertEqual(new Headers(calls[1]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SystemClient reads stats metrics prometheus cache and modules with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSystemClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/system/stats")) return jsonResponse({ requests_total: 12, tenants: 1 }); if (path.endsWith("/metrics")) return jsonResponse({ requests_total: 12 }); if (path.endsWith("/metrics/prometheus")) return new Response("yunque_requests_total 12\n", { status: 200, headers: { "Content-Type": "text/plain" } }); if (path.endsWith("/cache/stats")) return jsonResponse({ llm_response_cache: { entries: 2 } }); return jsonResponse({ modules: [{ id: "chat", enabled: true }], profile: "dev" }); } });
  const stats = await client.systemStats(); const metrics = await client.metrics(); const prometheus = await client.metricsPrometheus(); const cache = await client.cacheStats(); const modules = await client.modules();
  assertEqual(stats.requests_total, 12); assertEqual((metrics as { requests_total?: number }).requests_total, 12); assert(prometheus.includes("yunque_requests_total")); assertEqual((cache.llm_response_cache as { entries?: number }).entries, 2); assertEqual(modules.modules.length, 1);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});


test("SystemClient reads embedded SBOM metadata", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSystemClient({ baseUrl: "http://localhost:9090/", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ bomFormat: "CycloneDX", specVersion: "1.5", components: [{ name: "yunque-agent" }] }); } });
  const sbom = await client.sbom();
  assertEqual(sbom.bomFormat, "CycloneDX"); assertEqual(sbom.components?.length, 1); assertEqual(calls[0]?.url, "http://localhost:9090/sbom");
});

test("SystemClient throws SystemClientError with parsed and text bodies", async () => {
  const jsonClient = createSystemClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "invalid api key" }, { status: 401 }) });
  try { await jsonClient.metrics(); throw new Error("expected metrics to reject"); } catch (error) { assert(error instanceof SystemClientError); assertEqual(error.status, 401); assertDeepEqual(error.body, { error: "invalid api key" }); assertEqual(error.message, "invalid api key"); }
  const nestedClient = createSystemClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "system metrics scope is required" } }, { status: 400 }) });
  try { await nestedClient.metrics(); throw new Error("expected metrics to reject"); } catch (error) { assert(error instanceof SystemClientError); assertEqual(error.status, 400); assertEqual(error.message, "system metrics scope is required"); }
  const textClient = createSystemClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET only", { status: 405 }) });
  try { await textClient.modules(); throw new Error("expected modules to reject"); } catch (error) { assert(error instanceof SystemClientError); assertEqual(error.status, 405); assertEqual(error.body, "GET only"); assertEqual(error.message, "GET only"); }
});
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
