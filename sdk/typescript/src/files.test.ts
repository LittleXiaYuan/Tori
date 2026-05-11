import { createFilesClient, FilesClientError } from "./files";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FilesClient lists output files with bearer token and path query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ files: [{ name: "report.md", path: "drafts/report.md", size: 42, is_dir: false }] }); } });
  const result = await client.list("drafts");
  assertEqual(result.files[0]?.name, "report.md"); assertEqual(calls[0]?.url, "http://localhost:9090/api/files?path=drafts"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("FilesClient previews files with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ name: "report.md", path: "report.md", size: 12, ext: "md", kind: "text", content_type: "text/markdown", preview: "# Demo", truncated: false, editable: true, parse: { status: "parsed" } }); } });
  const preview = await client.preview("report.md");
  assertEqual(preview.kind, "text"); assertEqual(preview.preview, "# Demo"); assertEqual(calls[0]?.url, "http://localhost:9090/api/files/preview?path=report.md"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("FilesClient downloads artifacts as blobs with filename", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return new Response(new Blob(["hello"]), { status: 200, headers: { "Content-Type": "application/octet-stream", "Content-Disposition": 'attachment; filename="report.md"' } }); } });
  const result = await client.download("report.md");
  assertEqual(result.filename, "report.md"); assertEqual(result.contentType, "application/octet-stream"); assertEqual(await result.blob.text(), "hello"); assertEqual(calls[0]?.url, "http://localhost:9090/api/files/download?path=report.md");
});

test("FilesClient throws FilesClientError with parsed and text bodies", async () => {
  const jsonClient = createFilesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "path required" }, { status: 400 }) });
  try { await jsonClient.preview(""); throw new Error("expected preview to reject"); } catch (error) { assert(error instanceof FilesClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "path required" }); assertEqual(error.message, "path required"); }
  const nestedClient = createFilesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested path required" } }, { status: 400 }) });
  try { await nestedClient.preview(""); throw new Error("expected nested preview to reject"); } catch (error) { assert(error instanceof FilesClientError); assertEqual(error.status, 400); assertEqual(error.message, "nested path required"); }
  const textClient = createFilesClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("not found", { status: 404 }) });
  try { await textClient.download("missing.txt"); throw new Error("expected download to reject"); } catch (error) { assert(error instanceof FilesClientError); assertEqual(error.status, 404); assertEqual(error.body, "not found"); assertEqual(error.message, "not found"); }
});
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
