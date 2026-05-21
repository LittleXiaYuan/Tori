import { createSkillsScanClient, SkillsScanClientError } from "./skills-scan";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillsScanClient scans with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsScanClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "scanned", skills_loaded: 2, total_skills: 9 }); } });
  const scanned = await client.scan();
  assertEqual(scanned.skills_loaded, 2); assertEqual(calls[0]?.url, "http://localhost:9090/v1/skills/scan"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillsScanClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsScanClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "scanned" }); } });
  await client.scan();
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SkillsScanClient exposes nested scan errors", async () => {
  const client = createSkillsScanClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SKILLS_SCAN", message: "nested scan failed" } }, { status: 500 }) });
  try { await client.scan(); throw new Error("expected scan to reject"); } catch (error) { assert(error instanceof SkillsScanClientError); assertEqual(error.name, "SkillsClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "SKILLS_SCAN", message: "nested scan failed" } }); assertEqual(error.message, "nested scan failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
