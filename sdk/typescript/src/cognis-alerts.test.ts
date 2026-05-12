import { createCognisAlertsClient, CognisAlertsClientError } from "./cognis-alerts";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisAlertsClient lists and scans alerts with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisAlertsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ alerts: [{ severity: "high" }], count: 1 }); } });
  assertEqual((await client.list()).count, 1);
  assertEqual((await client.scan()).alerts?.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/alerts");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/alerts/scan");
  assertEqual(calls[1]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("CognisAlertsClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisAlertsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ count: 0 }); } });
  await client.scan();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CognisAlertsClient exposes nested alert errors", async () => {
  const client = createCognisAlertsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "ALERTS_DENIED", message: "alerts scan denied" } }, { status: 403 }) });
  try { await client.scan(); throw new Error("expected scan to reject"); } catch (error) { assert(error instanceof CognisAlertsClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: { code: "ALERTS_DENIED", message: "alerts scan denied" } }); assertEqual(error.message, "alerts scan denied"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
