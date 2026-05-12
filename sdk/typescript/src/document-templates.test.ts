import { createDocumentTemplatesClient, DocumentTemplatesClientError } from "./document-templates";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DocumentTemplatesClient lists templates with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDocumentTemplatesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ templates: [{ id: "brief", name: "Brief", format: "docx" }] }); } });
  const result = await client.templates();
  assertEqual(result.templates[0]?.id, "brief");
  assertEqual(result.templates[0]?.format, "docx");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/documents/templates");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("DocumentTemplatesClient supports API key auth", async () => {
  const calls: { init?: RequestInit }[] = [];
  const client = createDocumentTemplatesClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (_url, init) => { calls.push({ init }); return jsonResponse({ templates: [] }); } });
  assertDeepEqual((await client.templates()).templates, []);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("DocumentTemplatesClient exposes nested template errors", async () => {
  const client = createDocumentTemplatesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DOCUMENT_TEMPLATES", message: "templates failed" } }, { status: 503 }) });
  try { await client.templates(); throw new Error("expected templates to reject"); } catch (error) { assert(error instanceof DocumentTemplatesClientError); assertEqual(error.name, "DocumentsClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "DOCUMENT_TEMPLATES", message: "templates failed" } }); assertEqual(error.message, "templates failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
