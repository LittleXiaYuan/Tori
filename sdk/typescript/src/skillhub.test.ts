import { createSkillHubClient, SkillHubClientError } from "./skillhub";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillHubClient searches and trends skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("trending")) return jsonResponse({ skills: [{ name: "browser", source: "clawhub", installed: false }], count: 1, next_cursor: "n2" }); return jsonResponse({ results: [{ name: "browser", source: "local", installed: true }], count: 1 }); } });
  const search = await client.search({ q: "browser", limit: 5, source: "clawhub" }); const trending = await client.trending({ limit: 3, cursor: "n1" });
  assertEqual(search.count, 1); assertEqual(trending.next_cursor, "n2"); assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/search?q=browser&limit=5&source=clawhub"); assertEqual(calls[1]?.url, "http://localhost:9090/api/skillhub/trending?limit=3&cursor=n1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SkillHubClient manages install lifecycle with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("installed")) return jsonResponse({ skills: [{ slug: "browser", version: "1.0.0" }], count: 1 }); if (String(url).includes("uninstall")) return jsonResponse({ status: "uninstalled", slug: "browser" }); return jsonResponse({ status: "installed", slug: "browser", report: { score: 92 } }); } });
  const installed = await client.installed(); const install = await client.install("browser"); const uninstall = await client.uninstall("browser", "DELETE");
  assertEqual(installed.count, 1); assertEqual(install.status, "installed"); assertEqual(uninstall.status, "uninstalled"); assertEqual(calls[1]?.url, "http://localhost:9090/api/skillhub/install"); assertEqual(calls[2]?.init?.method, "DELETE"); assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { slug: "browser" });
});

test("SkillHubClient reads detail updates versions policy and analytics", async () => {
  const calls: string[] = [];
  const client = createSkillHubClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); const value = String(url); if (value.includes("detail")) return jsonResponse({ slug: "browser", name: "Browser", installed: true, security_score: 90 }); if (value.includes("check-updates")) return jsonResponse({ updates: [{ slug: "browser" }] }); if (value.includes("versions")) return jsonResponse({ versions: ["1.0.0"] }); if (value.includes("policy/check")) return jsonResponse({ allowed: true }); if (value.endsWith("/policy")) return jsonResponse({ min_security_score: 70 }); return jsonResponse({ total_skills: 2, installed_count: 1, security_stats: { high: 1 } }); } });
  const detail = await client.detail("browser"); const updates = await client.checkUpdates(); const versions = await client.versions("browser"); const policy = await client.policy(); const check = await client.policyCheck("browser"); const analytics = await client.analytics();
  assertEqual(detail.name, "Browser"); assertEqual(updates.updates.length, 1); assertEqual(versions.versions[0], "1.0.0"); assertEqual(policy.min_security_score, 70); assertEqual(check.allowed, true); assertEqual(analytics.installed_count, 1); assertEqual(calls[0], "http://localhost:9090/api/skillhub/detail?slug=browser");
});

test("SkillHubClient updates rollback and policy with JSON bodies", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, report: { score: 95 } }); } });
  await client.update("browser"); await client.rollback("browser", "1.0.0"); await client.updatePolicy({ min_security_score: 80 });
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { slug: "browser" }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { slug: "browser", version: "1.0.0" }); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { min_security_score: 80 });
});

test("SkillHubClient throws SkillHubClientError with parsed and text bodies", async () => {
  const auditClient = createSkillHubClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "audit failed", report: { score: 20 } }, { status: 422 }) });
  try { await auditClient.install("unsafe"); throw new Error("expected install to reject"); } catch (error) { assert(error instanceof SkillHubClientError); assertEqual(error.status, 422); assertDeepEqual(error.body, { error: "audit failed", report: { score: 20 } }); assertEqual(error.message, "audit failed"); }
  const textClient = createSkillHubClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("not found", { status: 404 }) });
  try { await textClient.detail("missing"); throw new Error("expected detail to reject"); } catch (error) { assert(error instanceof SkillHubClientError); assertEqual(error.status, 404); assertEqual(error.body, "not found"); assertEqual(error.message, "not found"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
