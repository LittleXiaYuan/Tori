import { createPersonaSkillsClient, PersonaSkillsClientError } from "./persona-skills";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PersonaSkillsClient lists persona skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaSkillsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ name: "review", content: "careful" }] }); } });
  const result = await client.skills();
  assertEqual(result.skills[0]?.name, "review");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/skills");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PersonaSkillsClient adds and deletes skills with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaSkillsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }, { status: init?.method === "POST" ? 201 : 200 }); } });
  assertEqual((await client.addSkill({ name: "planner", description: "Plan", content: "steps" })).status, "ok");
  assertEqual((await client.deleteSkill({ name: "planner" })).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/skills");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ name: "planner", description: "Plan", content: "steps" }));
  assertEqual(calls[1]?.init?.method, "DELETE");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ name: "planner" }));
  assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "key");
});

test("PersonaSkillsClient exposes nested skill errors", async () => {
  const client = createPersonaSkillsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "PERSONA_SKILL", message: "skill failed" } }, { status: 400 }) });
  try { await client.addSkill({ name: "" }); throw new Error("expected addSkill to reject"); } catch (error) { assert(error instanceof PersonaSkillsClientError); assertEqual(error.name, "PersonaClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "PERSONA_SKILL", message: "skill failed" } }); assertEqual(error.message, "skill failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
