import { createSkillHubVersionsClient, SkillHubVersionsClientError } from "./skillhub-versions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillHubVersionsClient lists versions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubVersionsClient({ baseUrl: "http://localhost:9090", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ versions: [{ version: "1.0.0" }] }); } });
  const result = await client.list("browser");
  assertEqual(result.versions.length, 1); assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/versions?slug=browser"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillHubVersionsClient rolls back with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubVersionsClient({ baseUrl: "http://localhost:9090/", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true }); } });
  const result = await client.rollback("browser", "1.0.0");
  assertEqual(result.ok, true); assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/rollback"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { slug: "browser", version: "1.0.0" }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("SkillHubVersionsClient exposes nested version errors", async () => {
  const client = createSkillHubVersionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested version failure" } }, { status: 404 }) });
  try { await client.list("missing"); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof SkillHubVersionsClientError); assertEqual(error.name, "SkillHubClientError"); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: { message: "nested version failure" } }); assertEqual(error.message, "nested version failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
