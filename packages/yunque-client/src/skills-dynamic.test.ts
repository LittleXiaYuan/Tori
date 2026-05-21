import { createSkillsDynamicClient, SkillsDynamicClientError } from "./skills-dynamic";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillsDynamicClient lists dynamic skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsDynamicClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "draft_doc", instruction: "write doc", approval_status: "pending" }] }); } });
  const dynamic = await client.list();
  assertEqual(dynamic.skills[0]?.approval_status, "pending"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/skills/dynamic"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillsDynamicClient approves and rejects with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsDynamicClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.approve({ name: "draft_doc", instruction: "use safely" })).status, "ok"); assertEqual((await client.reject("old_skill")).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/skills/approve"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "draft_doc", instruction: "use safely" }); assertEqual(calls[1]?.url, "http://localhost:9090/v1/skills/reject"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { name: "old_skill" }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SkillsDynamicClient exposes nested dynamic errors", async () => {
  const client = createSkillsDynamicClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SKILLS_DYNAMIC", message: "nested dynamic failed" } }, { status: 400 }) });
  try { await client.approve({ name: "" }); throw new Error("expected approve to reject"); } catch (error) { assert(error instanceof SkillsDynamicClientError); assertEqual(error.name, "SkillsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "SKILLS_DYNAMIC", message: "nested dynamic failed" } }); assertEqual(error.message, "nested dynamic failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
