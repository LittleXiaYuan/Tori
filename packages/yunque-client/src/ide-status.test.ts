import { createIDEStatusClient, IDEStatusClientError } from "./ide-status";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("IDEStatusClient reads status with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIDEStatusClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ version: "0.1.0", connected: true, capabilities: ["review"], skills_count: 3 }); } });
  const result = await client.status();
  assertEqual(result.connected, true);
  assertEqual(result.capabilities?.[0], "review");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/ide/status");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("IDEStatusClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIDEStatusClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ connected: false, uptime_sec: 12 }); } });
  assertEqual((await client.status()).uptime_sec, 12);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("IDEStatusClient exposes nested status errors", async () => {
  const client = createIDEStatusClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "IDE_STATUS", message: "ide offline" } }, { status: 503 }) });
  try { await client.status(); throw new Error("expected status to reject"); } catch (error) { assert(error instanceof IDEStatusClientError); assertEqual(error.name, "IDEClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "IDE_STATUS", message: "ide offline" } }); assertEqual(error.message, "ide offline"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
