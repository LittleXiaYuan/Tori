import { createFilesDownloadClient, FilesDownloadClientError } from "./files-download";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FilesDownloadClient downloads artifacts as blobs with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesDownloadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return new Response(new Blob(["hello"]), { status: 200, headers: { "Content-Type": "application/octet-stream", "Content-Disposition": 'attachment; filename="report.md"' } }); } });
  const result = await client.download("report.md");
  assertEqual(result.filename, "report.md");
  assertEqual(result.contentType, "application/octet-stream");
  assertEqual(await result.blob.text(), "hello");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/files/download?path=report.md");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FilesDownloadClient supports API key auth and RFC5987 filename", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesDownloadClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return new Response(new Blob(["doc"]), { status: 200, headers: { "Content-Type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "Content-Disposition": "attachment; filename*=UTF-8''%E6%8A%A5%E5%91%8A.docx" } }); } });
  const result = await client.download("报告.docx");
  assertEqual(result.filename, "报告.docx");
  assertEqual(await result.blob.text(), "doc");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/files/download?path=%E6%8A%A5%E5%91%8A.docx");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FilesDownloadClient exposes nested download errors", async () => {
  const client = createFilesDownloadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FILES_DOWNLOAD", message: "not found" } }, { status: 404 }) });
  try { await client.download("missing.txt"); throw new Error("expected download to reject"); } catch (error) { assert(error instanceof FilesDownloadClientError); assertEqual(error.name, "FilesClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { code: "FILES_DOWNLOAD", message: "not found" } }); assertEqual(error.message, "not found"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
