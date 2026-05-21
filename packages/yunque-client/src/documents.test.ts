import { createDocumentsClient, DocumentsClientError } from "./documents";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
}

const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];

function test(name: string, fn: () => Promise<void> | void): void {
  tests.push({ name, fn });
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

test("DocumentsClient lists templates with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDocumentsClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ templates: [{ id: "brief", format: "docx" }] });
    },
  });

  const result = await client.templates();

  assertEqual(result.templates[0]?.id, "brief");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/documents/templates");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("DocumentsClient generates documents with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDocumentsClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ result: "created", path: "data/output/roadmap.docx", format: "docx" });
    },
  });

  const result = await client.generate({
    format: "docx",
    title: "技术蓝图",
    content: "# 云雀技术蓝图",
    path: "data/output/roadmap.docx",
  });

  assertEqual(result.path, "data/output/roadmap.docx");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/documents/generate");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(
    calls[0]?.init?.body,
    JSON.stringify({ format: "docx", title: "技术蓝图", content: "# 云雀技术蓝图", path: "data/output/roadmap.docx" }),
  );
});

test("DocumentsClient provides format helpers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDocumentsClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      const body = JSON.parse(String(init?.body));
      return jsonResponse({ result: "created", path: `data/output/demo.${body.format}`, format: body.format });
    },
  });

  await client.generateXlsx({ content: "a,b\n1,2", sheet_name: "数据" });
  await client.generatePptx({ content: "# 路演" });
  await client.generateHtml({ content: "<h1>Demo</h1>" });

  assertEqual(JSON.parse(String(calls[0]?.init?.body)).format, "xlsx");
  assertEqual(JSON.parse(String(calls[0]?.init?.body)).sheet_name, "数据");
  assertEqual(JSON.parse(String(calls[1]?.init?.body)).format, "pptx");
  assertEqual(JSON.parse(String(calls[2]?.init?.body)).format, "html");
});

test("DocumentsClient throws DocumentsClientError with parsed body", async () => {
  const client = createDocumentsClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "unsupported format: pdf" }, { status: 400 }),
  });

  try {
    await client.generate({ format: "pdf", content: "demo" });
    throw new Error("expected generate to reject");
  } catch (error) {
    assert(error instanceof DocumentsClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "unsupported format: pdf" });
    assertEqual(error.message, "unsupported format: pdf");
  }

  const nestedClient = createDocumentsClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "document content is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.generate({ format: "docx", content: "" });
    throw new Error("expected generate to reject");
  } catch (error) {
    assert(error instanceof DocumentsClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "document content is required");
  }
});

let failures = 0;
for (const { name, fn } of tests) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failures > 0) {
  process.exitCode = 1;
} else {
  console.log(`1..${tests.length}`);
}
