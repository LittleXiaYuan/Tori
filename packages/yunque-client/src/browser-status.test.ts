import { createBrowserStatusClient, BrowserStatusClientError } from "./browser-status";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BrowserStatusClient reads browser status and config with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserStatusClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/status")) return jsonResponse({ connected: true, state: "extension", version: "1.0.0" }); return jsonResponse({ mode: "extension", connected: true, headless: false }); } });
  const status = await client.status();
  const config = await client.config();
  assertEqual(status.connected, true);
  assertEqual(config.mode, "extension");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/browser/status");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/browser/config");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("BrowserStatusClient reads extension status with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserStatusClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ connected: true, pending: 2 }); } });
  const status = await client.extensionStatus();
  assertEqual(status.pending, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/browser/ext/status");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("BrowserStatusClient exposes nested browser errors", async () => {
  const client = createBrowserStatusClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested browser status failure" } }, { status: 400 }) });
  try { await client.status(); throw new Error("expected status to reject"); } catch (error) { assert(error instanceof BrowserStatusClientError); assertEqual(error.name, "BrowserClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested browser status failure" } }); assertEqual(error.message, "nested browser status failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
