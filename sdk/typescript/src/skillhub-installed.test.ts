import { createSkillHubInstalledClient, SkillHubInstalledClientError } from "./skillhub-installed";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillHubInstalledClient lists installed skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubInstalledClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ slug: "browser", version: "1.0.0" }], count: 1 }); } });
  const result = await client.list();
  assertEqual(result.count, 1); assertEqual(result.skills[0]?.slug, "browser"); assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/installed"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillHubInstalledClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubInstalledClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [], count: 0 }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("SkillHubInstalledClient exposes nested installed errors", async () => {
  const client = createSkillHubInstalledClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "nested installed failure" } }, { status: 500 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof SkillHubInstalledClientError); assertEqual(error.name, "SkillHubClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { message: "nested installed failure" } }); assertEqual(error.message, "nested installed failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
