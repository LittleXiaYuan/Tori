import { createSkillsSuggestionsClient, SkillsSuggestionsClientError } from "./skills-suggestions";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillsSuggestionsClient fetches session suggestions with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsSuggestionsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ suggestions: [{ name: "summarize" }] }); } });
  const suggestions = await client.suggestions("sess-1");
  assertEqual(suggestions.suggestions[0]?.name, "summarize"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/skill-suggestions?session_id=sess-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillsSuggestionsClient supports API key auth and empty query", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillsSuggestionsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ suggestions: [] }); } });
  await client.suggestions();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/skill-suggestions"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SkillsSuggestionsClient exposes nested suggestion errors", async () => {
  const client = createSkillsSuggestionsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SKILLS_SUGGEST", message: "nested suggestion failed" } }, { status: 400 }) });
  try { await client.suggestions("bad"); throw new Error("expected suggestions to reject"); } catch (error) { assert(error instanceof SkillsSuggestionsClientError); assertEqual(error.name, "SkillsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "SKILLS_SUGGEST", message: "nested suggestion failed" } }); assertEqual(error.message, "nested suggestion failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
