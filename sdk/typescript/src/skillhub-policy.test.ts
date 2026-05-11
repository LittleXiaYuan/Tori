import { createSkillHubPolicyClient, SkillHubPolicyClientError } from "./skillhub-policy";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillHubPolicyClient reads policy with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubPolicyClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ min_security_score: 70, allow_sources: ["local"] }); } });
  const result = await client.policy();
  assertEqual(result.min_security_score, 70);
  assertDeepEqual(result.allow_sources, ["local"]);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/policy");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillHubPolicyClient updates policy and checks slug with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubPolicyClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/check")) return jsonResponse({ allowed: true, reason: "ok" }); return jsonResponse({ ok: true }); } });
  const updated = await client.updatePolicy({ min_security_score: 80, require_signature: true });
  const checked = await client.check("browser");
  assertEqual(updated.ok, true);
  assertEqual(checked.allowed, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/policy");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { min_security_score: 80, require_signature: true });
  assertEqual(calls[1]?.url, "http://localhost:9090/api/skillhub/policy/check?slug=browser");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SkillHubPolicyClient reads analytics", async () => {
  const calls: string[] = [];
  const client = createSkillHubPolicyClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); return jsonResponse({ total_skills: 10, installed_count: 3, security_stats: { high: 2 } }); } });
  const result = await client.analytics();
  assertEqual(result.total_skills, 10);
  assertEqual(result.installed_count, 3);
  assertDeepEqual(result.security_stats, { high: 2 });
  assertEqual(calls[0], "http://localhost:9090/api/skillhub/analytics");
});

test("SkillHubPolicyClient exposes policy nested gateway errors", async () => {
  const client = createSkillHubPolicyClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested policy failure" } }, { status: 400 }) });
  try { await client.check(""); throw new Error("expected check to reject"); } catch (error) { assert(error instanceof SkillHubPolicyClientError); assertEqual(error.name, "SkillHubClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested policy failure" } }); assertEqual(error.message, "nested policy failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
