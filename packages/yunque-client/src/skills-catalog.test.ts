import { createSkillsCatalogClient, SkillsCatalogClientError } from "./skills-catalog";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillsCatalogClient lists runtime skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsCatalogClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "web.search", description: "search" }], count: 1, categories: [{ id: "web", name: "Web" }] }); } });
  const result = await client.list();
  assertEqual(result.count, 1); assertEqual(result.skills[0]?.name, "web.search"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/skills"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillsCatalogClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsCatalogClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [], count: 0 }); } });
  await client.list();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SkillsCatalogClient exposes nested catalog errors", async () => {
  const client = createSkillsCatalogClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SKILLS_CATALOG", message: "nested catalog failed" } }, { status: 500 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof SkillsCatalogClientError); assertEqual(error.name, "SkillsClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "SKILLS_CATALOG", message: "nested catalog failed" } }); assertEqual(error.message, "nested catalog failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
