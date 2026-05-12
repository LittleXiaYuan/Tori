import { createSetupTemplatesClient, SetupTemplatesClientError } from "./setup-templates";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SetupTemplatesClient lists templates with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupTemplatesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ templates: [{ id: "local-first", name: "Local First", sandbox_tier: "local" }], count: 1 }); } });
  const templates = await client.list();
  assertEqual(templates.count, 1);
  assertEqual(templates.templates[0]?.id, "local-first");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/templates");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SetupTemplatesClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupTemplatesClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ templates: [], count: 0 }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SetupTemplatesClient exposes nested setup errors", async () => {
  const client = createSetupTemplatesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested setup template failure" } }, { status: 400 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof SetupTemplatesClientError); assertEqual(error.name, "SetupClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested setup template failure" } }); assertEqual(error.message, "nested setup template failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
