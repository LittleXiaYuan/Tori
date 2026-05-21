import { createCognisBundlesClient, CognisBundlesClientError } from "./cognis-bundles";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisBundlesClient generates and exports bundles with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisBundlesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/export")) return jsonResponse({ bundle: { version: 1 } }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.generate({ target: "all" })).status, "ok");
  assertDeepEqual((await client.export() as { bundle?: Record<string, unknown> }).bundle, { version: 1 });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/generate");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/export");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { target: "all" });
});

test("CognisBundlesClient imports bundles with API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisBundlesClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.import({ cognis: [{ id: "doc" }] });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/import");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { cognis: [{ id: "doc" }] });
});

test("CognisBundlesClient exposes nested bundle errors", async () => {
  const client = createCognisBundlesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BUNDLE", message: "bundle failed" } }, { status: 422 }) });
  try { await client.generate({}); throw new Error("expected generate to reject"); } catch (error) { assert(error instanceof CognisBundlesClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 422); assertDeepEqual(error.body, { error: { code: "BUNDLE", message: "bundle failed" } }); assertEqual(error.message, "bundle failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
