import { createBrowserCaptureClient, BrowserCaptureClientError } from "./browser-capture";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BrowserCaptureClient reads screenshots with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserCaptureClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ screenshot: "abc", timestamp: "2026-05-12T00:00:00Z" }); } });
  const shot = await client.screenshot();
  const latest = await client.latestScreenshot();
  assertEqual(shot.screenshot, "abc");
  assertEqual(latest.timestamp, "2026-05-12T00:00:00Z");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/browser/screenshot");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/browser/screenshot/latest");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("BrowserCaptureClient runs OCR with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBrowserCaptureClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ text: "page text", result: "page text" }); } });
  const result = await client.ocr();
  assertEqual(result.text, "page text");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/browser/ocr");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("BrowserCaptureClient exposes nested browser capture errors", async () => {
  const client = createBrowserCaptureClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested browser capture failure" } }, { status: 400 }) });
  try { await client.screenshot(); throw new Error("expected screenshot to reject"); } catch (error) { assert(error instanceof BrowserCaptureClientError); assertEqual(error.name, "BrowserClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested browser capture failure" } }); assertEqual(error.message, "nested browser capture failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
