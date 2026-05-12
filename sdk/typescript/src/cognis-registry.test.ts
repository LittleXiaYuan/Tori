import { createCognisRegistryClient, CognisRegistryClientError } from "./cognis-registry";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisRegistryClient manages registry entries with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisRegistryClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/v1/cognis") && init?.method === "GET") return jsonResponse({ cognis: [{ id: "doc", name: "文档助手" }], count: 1 }); if (path.endsWith("/enable") || path.endsWith("/disable") || path.endsWith("/reload") || init?.method === "DELETE") return jsonResponse({ status: "ok" }); return jsonResponse({ id: "doc", name: "文档助手", enabled: true }); } });
  assertEqual((await client.list()).count, 1);
  assertEqual((await client.create({ id: "doc", name: "文档助手" })).id, "doc");
  assertEqual((await client.get("doc/id")).enabled, true);
  assertEqual((await client.enable("doc/id")).status, "ok");
  assertEqual((await client.disable("doc/id")).status, "ok");
  assertEqual((await client.remove("doc/id")).status, "ok");
  assertEqual((await client.reload()).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "doc", name: "文档助手" });
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cognis/doc%2Fid");
});

test("CognisRegistryClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisRegistryClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.reload();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/reload");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CognisRegistryClient exposes nested registry errors", async () => {
  const client = createCognisRegistryClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "cogni id is required" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof CognisRegistryClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "cogni id is required" } }); assertEqual(error.message, "cogni id is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
