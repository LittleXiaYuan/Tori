import { createSkillsClient, SkillsClientError } from "./skills";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillsClient lists runtime skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "web.search", description: "search", usage_total: 3, success_rate: 1 }], count: 1, categories: [{ id: "web", name: "Web" }] }); } });
  const result = await client.list();
  assertEqual(result.count, 1); assertEqual(result.skills[0]?.name, "web.search"); assertEqual(result.categories?.[0]?.id, "web"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/skills"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillsClient scans and reads dynamic skills with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/scan")) return jsonResponse({ status: "scanned", skills_loaded: 2, total_skills: 9 }); return jsonResponse({ skills: [{ name: "draft_doc", instruction: "write doc", approval_status: "pending" }] }); } });
  const scanned = await client.scan(); const dynamic = await client.dynamic();
  assertEqual(scanned.skills_loaded, 2); assertEqual(dynamic.skills[0]?.approval_status, "pending"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/skills/scan"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/skills/dynamic"); assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "key");
});

test("SkillsClient approves rejects and fetches session suggestions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("skill-suggestions")) return jsonResponse({ suggestions: [{ name: "summarize" }] }); return jsonResponse({ status: "ok" }); } });
  const approved = await client.approve({ name: "draft_doc", instruction: "use safely" }); const rejected = await client.reject("old_skill"); const suggestions = await client.suggestions("sess-1");
  assertEqual(approved.status, "ok"); assertEqual(rejected.status, "ok"); assertEqual(suggestions.suggestions[0]?.name, "summarize");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/skills/approve"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "draft_doc", instruction: "use safely" });
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/skills/reject"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { name: "old_skill" });
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/skill-suggestions?session_id=sess-1");
});

test("SkillsClient throws SkillsClientError with parsed and text bodies", async () => {
  const jsonClient = createSkillsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "skill registry not configured" }, { status: 500 }) });
  try { await jsonClient.dynamic(); throw new Error("expected dynamic to reject"); } catch (error) { assert(error instanceof SkillsClientError); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: "skill registry not configured" }); assertEqual(error.message, "skill registry not configured"); }
  const textClient = createSkillsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST only", { status: 405 }) });
  try { await textClient.scan(); throw new Error("expected scan to reject"); } catch (error) { assert(error instanceof SkillsClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST only"); assertEqual(error.message, "POST only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
