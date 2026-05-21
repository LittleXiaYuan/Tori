import { createDocumentGenerateClient, DocumentGenerateClientError } from "./document-generate";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DocumentGenerateClient generates explicit format with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDocumentGenerateClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ result: "created", path: "data/output/brief.docx", format: "docx" }); } });
  const result = await client.generate({ format: "docx", title: "技术蓝图", content: "# 云雀", path: "data/output/brief.docx" });
  assertEqual(result.path, "data/output/brief.docx");
  assertEqual(result.format, "docx");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/documents/generate");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ format: "docx", title: "技术蓝图", content: "# 云雀", path: "data/output/brief.docx" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("DocumentGenerateClient provides format helpers with API key", async () => {
  const calls: { init?: RequestInit }[] = [];
  const client = createDocumentGenerateClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (_url, init) => { calls.push({ init }); const body = JSON.parse(String(init?.body)); return jsonResponse({ result: "created", path: `data/output/demo.${body.format}`, format: body.format }); } });
  await client.generateXlsx({ content: "a,b\n1,2", sheet_name: "数据" });
  await client.generatePptx({ content: "# 路演" });
  await client.generateHtml({ content: "<h1>Demo</h1>" });
  assertEqual(JSON.parse(String(calls[0]?.init?.body)).format, "xlsx");
  assertEqual(JSON.parse(String(calls[0]?.init?.body)).sheet_name, "数据");
  assertEqual(JSON.parse(String(calls[1]?.init?.body)).format, "pptx");
  assertEqual(JSON.parse(String(calls[2]?.init?.body)).format, "html");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("DocumentGenerateClient exposes nested generate errors", async () => {
  const client = createDocumentGenerateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DOCUMENT_GENERATE", message: "generate failed" } }, { status: 400 }) });
  try { await client.generate({ format: "docx", content: "" }); throw new Error("expected generate to reject"); } catch (error) { assert(error instanceof DocumentGenerateClientError); assertEqual(error.name, "DocumentsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "DOCUMENT_GENERATE", message: "generate failed" } }); assertEqual(error.message, "generate failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
