import { createCognisHealthClient, CognisHealthClientError } from "./cognis-health";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisHealthClient reads stats health and verify with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisHealthClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, path: String(url) }); } });
  assertEqual((await client.stats()).ok, true);
  assertEqual((await client.health("cogni/id")).ok, true);
  assertEqual((await client.verify("cogni/id")).ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/stats");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/cogni%2Fid/health");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cognis/cogni%2Fid/verify");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CognisHealthClient supports global health with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisHealthClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.health(); await client.verify();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/health");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/verify");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CognisHealthClient exposes nested health errors", async () => {
  const client = createCognisHealthClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "UNHEALTHY", message: "cogni health unavailable" } }, { status: 503 }) });
  try { await client.health("bad"); throw new Error("expected health to reject"); } catch (error) { assert(error instanceof CognisHealthClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "UNHEALTHY", message: "cogni health unavailable" } }); assertEqual(error.message, "cogni health unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
