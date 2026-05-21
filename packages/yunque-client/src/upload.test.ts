import { createUploadClient, UploadClientError } from "./upload";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("UploadClient uploads multipart files with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createUploadClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ filename: "apply.docx", size: 12, path: "apply.docx", parse: { status: "parsed", parser: "local", preview: "公司名称\t云雀" } }); } });
  const uploaded = await client.upload({ file: new Blob(["hello"], { type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document" }), filename: "apply.docx" });
  assertEqual(uploaded.filename, "apply.docx"); assertEqual(uploaded.parse?.status, "parsed"); assertEqual(uploaded.parse?.preview, "公司名称\t云雀");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/upload"); assertEqual(calls[0]?.init?.method, "POST"); assert(calls[0]?.init?.body instanceof FormData); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), null, "multipart boundary must be set by fetch");
});

test("UploadClient supports file alias and API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createUploadClient({ baseUrl: "http://localhost:9090", apiKey: "key-1", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ filename: "slides.pptx", size: 7, path: "slides.pptx", parse: { status: "needs_document_parser", parser: "document", note: "等待文档解析后端展开正文" } }); } });
  const uploaded = await client.file(new Blob(["slides"]), "slides.pptx");
  assertEqual(uploaded.parse?.status, "needs_document_parser"); assertEqual(uploaded.parse?.parser, "document"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-1");
});

test("UploadClient preserves analysis actions and rich payload", async () => {
  const client = createUploadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ filename: "plan.txt", size: 4, path: "plan.txt", analysis: { kind: "template" }, actions: [{ type: "write_file", path: "plan.md" }], rich: { blocks: [] } }) });
  const uploaded = await client.upload({ file: new Blob(["plan"]), filename: "plan.txt" });
  assertEqual(uploaded.actions?.[0]?.type, "write_file"); assertDeepEqual(uploaded.analysis, { kind: "template" }); assertDeepEqual(uploaded.rich, { blocks: [] });
});

test("UploadClient throws UploadClientError with parsed and text bodies", async () => {
  const jsonClient = createUploadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "file field required" }, { status: 400 }) });
  try { await jsonClient.upload({ file: new Blob([]) }); throw new Error("expected upload to reject"); } catch (error) { assert(error instanceof UploadClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "file field required" }); assertEqual(error.message, "file field required"); }
  const nestedClient = createUploadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested file field required" } }, { status: 400 }) });
  try { await nestedClient.upload({ file: new Blob([]) }); throw new Error("expected nested upload to reject"); } catch (error) { assert(error instanceof UploadClientError); assertEqual(error.status, 400); assertEqual(error.message, "nested file field required"); }
  const textClient = createUploadClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST only", { status: 405 }) });
  try { await textClient.file(new Blob(["x"]), "x.txt"); throw new Error("expected file to reject"); } catch (error) { assert(error instanceof UploadClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST only"); assertEqual(error.message, "POST only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
