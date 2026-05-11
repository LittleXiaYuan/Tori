import { createCognisClient, CognisClientError } from "./cognis";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisClient manages registry entries with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/v1/cognis") && init?.method === "GET") return jsonResponse({ cognis: [{ id: "doc", name: "文档助手" }], count: 1 }); if (path.endsWith("/enable") || path.endsWith("/disable")) return jsonResponse({ status: "ok" }); return jsonResponse({ id: "doc", name: "文档助手", enabled: true }); } });
  const list = await client.list(); const created = await client.create({ id: "doc", name: "文档助手" }); const detail = await client.get("doc"); const enabled = await client.enable("doc"); const disabled = await client.disable("doc");
  assertEqual(list.count, 1); assertEqual(created.id, "doc"); assertEqual(detail.enabled, true); assertEqual(enabled.status, "ok"); assertEqual(disabled.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "doc", name: "文档助手" });
});

test("CognisClient reads traces health verify alerts and reloads", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.includes("traces") || path.includes("/trace")) return jsonResponse({ traces: [{ id: "evt" }], count: 1 }); if (path.includes("health")) return jsonResponse({ healthy: true }); if (path.includes("verify")) return jsonResponse({ ok: true }); if (path.includes("alerts")) return jsonResponse({ alerts: [], count: 0 }); return jsonResponse({ status: "ok" }); } });
  const traces = await client.traces(5); const trace = await client.trace("doc", 2); const health = await client.health("doc"); const verify = await client.verify(); const alerts = await client.alerts(); const scanned = await client.scanAlerts(); const reload = await client.reload();
  assertEqual(traces.count, 1); assertEqual(trace.count, 1); assertEqual((health as { healthy?: boolean }).healthy, true); assertEqual((verify as { ok?: boolean }).ok, true); assertEqual(alerts.count, 0); assertEqual(scanned.count, 0); assertEqual(reload.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/traces?limit=5"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/doc/trace?limit=2"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CognisClient controls bundles workflows evolution and federation", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/export")) return jsonResponse({ bundle: { version: 1 } }); if (path.includes("federation/peers")) return jsonResponse({ peers: [] }); if (path.endsWith("/economics")) return jsonResponse({ cost: 0 }); return jsonResponse({ status: "ok" }); } });
  const generated = await client.generate({ prompt: "make cogni" }); const exported = await client.exportBundle(); const imported = await client.importBundle({ bundle: { version: 1 } }); const workflows = await client.workflows("doc"); const ran = await client.runWorkflow("doc", "summarize", { input: "x" }); const evolved = await client.evolve("doc"); const evolution = await client.evolution(); const federation = await client.federation(); const peers = await client.federationPeers(); const discovered = await client.discoverFederation({ query: "doc" }); const exposed = await client.expose("doc"); const unexposed = await client.unexpose("doc"); const economics = await client.economics();
  assertEqual(generated.status, "ok"); assertDeepEqual(exported, { bundle: { version: 1 } }); assertEqual(imported.status, "ok"); assertEqual((workflows as { status?: string }).status, "ok"); assertEqual((ran as { status?: string }).status, "ok"); assertEqual(evolved.status, "ok"); assertEqual((evolution as { status?: string }).status, "ok"); assertEqual((federation as { status?: string }).status, "ok"); assertDeepEqual(peers, { peers: [] }); assertEqual((discovered as { status?: string }).status, "ok"); assertEqual(exposed.status, "ok"); assertEqual(unexposed.status, "ok"); assertDeepEqual(economics, { cost: 0 });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/generate"); assertEqual(calls[4]?.url, "http://localhost:9090/v1/cognis/doc/workflow/summarize"); assertDeepEqual(JSON.parse(String(calls[4]?.init?.body)), { input: "x" });
});

test("CognisClient throws CognisClientError with parsed and text bodies", async () => {
  const jsonClient = createCognisClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "cogni registry not configured" }, { status: 500 }) });
  try { await jsonClient.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof CognisClientError); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: "cogni registry not configured" }); assertEqual(error.message, "cogni registry not configured"); }
  const nestedClient = createCognisClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "cogni id is required" } }, { status: 400 }) });
  try { await nestedClient.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof CognisClientError); assertEqual(error.status, 400); assertEqual(error.message, "cogni id is required"); }
  const textClient = createCognisClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("GET only", { status: 405 }) });
  try { await textClient.health(); throw new Error("expected health to reject"); } catch (error) { assert(error instanceof CognisClientError); assertEqual(error.status, 405); assertEqual(error.body, "GET only"); assertEqual(error.message, "GET only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
