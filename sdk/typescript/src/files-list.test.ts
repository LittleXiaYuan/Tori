import { createFilesListClient, FilesListClientError } from "./files-list";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("FilesListClient lists files with bearer token and path query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesListClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ files: [{ name: "report.md", path: "drafts/report.md", size: 42, is_dir: false }] }); } });
  const result = await client.list("drafts");
  assertEqual(result.files[0]?.name, "report.md"); assertEqual(calls[0]?.url, "http://localhost:9090/api/files?path=drafts"); assertEqual(calls[0]?.init?.method, "GET"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("FilesListClient supports API key auth and empty path", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createFilesListClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ files: [] }); } });
  await client.list();
  assertEqual(calls[0]?.url, "http://localhost:9090/api/files"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("FilesListClient exposes nested list errors", async () => {
  const client = createFilesListClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FILES_LIST", message: "nested list failed" } }, { status: 400 }) });
  try { await client.list("bad"); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof FilesListClientError); assertEqual(error.name, "FilesClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "FILES_LIST", message: "nested list failed" } }); assertEqual(error.message, "nested list failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
