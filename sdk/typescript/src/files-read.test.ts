import { createFilesReadClient, FilesReadClientError } from "./files-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FilesReadClient lists files with bearer token and path query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesReadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ files: [{ name: "report.md", path: "drafts/report.md", size: 42, is_dir: false }] }); } });
  const result = await client.list("drafts");
  assertEqual(result.files[0]?.name, "report.md");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/files?path=drafts");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FilesReadClient previews files with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesReadClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ name: "report.md", path: "report.md", size: 12, ext: "md", kind: "text", content_type: "text/markdown", preview: "# Demo", truncated: false, editable: true }); } });
  const preview = await client.preview("report.md");
  assertEqual(preview.kind, "text");
  assertEqual(preview.preview, "# Demo");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/files/preview?path=report.md");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FilesReadClient exposes nested read errors", async () => {
  const client = createFilesReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FILES_READ", message: "path required" } }, { status: 400 }) });
  try { await client.preview(""); throw new Error("expected preview to reject"); } catch (error) { assert(error instanceof FilesReadClientError); assertEqual(error.name, "FilesClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "FILES_READ", message: "path required" } }); assertEqual(error.message, "path required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
