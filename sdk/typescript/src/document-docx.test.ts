import { createDocumentDocxClient, DocumentDocxClientError } from "./document-docx";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DocumentDocxClient generates DOCX with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDocumentDocxClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ result: "created", path: "data/output/demo.docx", format: "docx" }); } });
  const result = await client.generate({ title: "技术蓝图", content: "# 云雀", path: "data/output/demo.docx" });
  assertEqual(result.format, "docx"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/documents/generate"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  const body = JSON.parse(String(calls[0]?.init?.body)); assertEqual(body.format, "docx"); assertEqual(body.title, "技术蓝图");
});

test("DocumentDocxClient supports API key auth", async () => {
  const calls: { init?: RequestInit }[] = [];
  const client = createDocumentDocxClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (_url, init) => { calls.push({ init }); return jsonResponse({ result: "created", path: "data/output/demo.docx", format: "docx" }); } });
  await client.generate({ content: "demo" });
  assertEqual(JSON.parse(String(calls[0]?.init?.body)).format, "docx"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("DocumentDocxClient exposes nested docx errors", async () => {
  const client = createDocumentDocxClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DOCUMENT_DOCX", message: "nested docx generation failed" } }, { status: 400 }) });
  try { await client.generate({ content: "" }); throw new Error("expected generate to reject"); } catch (error) { assert(error instanceof DocumentDocxClientError); assertEqual(error.name, "DocumentsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "DOCUMENT_DOCX", message: "nested docx generation failed" } }); assertEqual(error.message, "nested docx generation failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
