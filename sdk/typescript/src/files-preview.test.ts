import { createFilesPreviewClient, FilesPreviewClientError } from "./files-preview";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FilesPreviewClient previews files with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesPreviewClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ name: "report.md", path: "report.md", size: 12, ext: "md", kind: "text", preview: "# Demo" }); } });
  const preview = await client.preview("report.md");
  assertEqual(preview.kind, "text"); assertEqual(preview.preview, "# Demo"); assertEqual(calls[0]?.url, "http://localhost:9090/api/files/preview?path=report.md"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FilesPreviewClient supports API key auth and encoded path", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesPreviewClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ name: "报告.md", path: "报告.md", size: 12, preview: "ok" }); } });
  await client.preview("报告.md");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/files/preview?path=%E6%8A%A5%E5%91%8A.md"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FilesPreviewClient exposes nested preview errors", async () => {
  const client = createFilesPreviewClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FILES_PREVIEW", message: "nested preview failed" } }, { status: 400 }) });
  try { await client.preview(""); throw new Error("expected preview to reject"); } catch (error) { assert(error instanceof FilesPreviewClientError); assertEqual(error.name, "FilesClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "FILES_PREVIEW", message: "nested preview failed" } }); assertEqual(error.message, "nested preview failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
