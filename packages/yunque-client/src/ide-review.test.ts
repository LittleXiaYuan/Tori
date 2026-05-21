import { createIDEReviewClient, IDEReviewClientError } from "./ide-review";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("IDEReviewClient reviews explicit body with bearer token", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const client = createIDEReviewClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); return jsonResponse({ summary: "ok", issues: [{ line: 1, severity: "info", message: "fine" }], score: 9 }); } });
  const result = await client.review({ file_path: "main.go", content: "package main", language: "go", mode: "full" });
  assertEqual(result.score, 9);
  assertEqual(result.issues?.[0]?.line, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/ide/review");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), "application/json");
  assertDeepEqual(calls[0]?.body, { file_path: "main.go", content: "package main", language: "go", mode: "full" });
});

test("IDEReviewClient provides review helpers with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIDEReviewClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ summary: "ok", issues: [], improvements: ["test"] }); } });
  await client.reviewDiff({ file_path: "main.go", diff: "+fmt.Println(1)", language: "go" });
  await client.reviewQuick({ file_path: "main.ts", content: "console.log(1)", language: "ts" });
  await client.reviewFull({ file_path: "main.py", content: "print(1)", language: "py" });
  assertEqual(JSON.parse(String(calls[0]?.init?.body)).mode, "diff");
  assertEqual(JSON.parse(String(calls[1]?.init?.body)).mode, "quick");
  assertEqual(JSON.parse(String(calls[2]?.init?.body)).mode, "full");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("IDEReviewClient exposes nested review errors", async () => {
  const client = createIDEReviewClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "IDE_REVIEW", message: "content required" } }, { status: 400 }) });
  try { await client.review({}); throw new Error("expected review to reject"); } catch (error) { assert(error instanceof IDEReviewClientError); assertEqual(error.name, "IDEClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "IDE_REVIEW", message: "content required" } }); assertEqual(error.message, "content required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
