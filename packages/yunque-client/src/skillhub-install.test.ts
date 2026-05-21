import { createSkillHubInstallClient, SkillHubInstallClientError } from "./skillhub-install";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillHubInstallClient lists installed skills with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubInstallClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ skills: [{ slug: "browser", version: "1.0.0" }], count: 1 }); } });
  const result = await client.installed();
  assertEqual(result.count, 1);
  assertEqual(result.skills[0]?.slug, "browser");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/installed");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillHubInstallClient installs and uninstalls skills with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubInstallClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("uninstall")) return jsonResponse({ status: "uninstalled", slug: "browser" }); return jsonResponse({ status: "installed", slug: "browser", report: { score: 96 } }); } });
  const installed = await client.install("browser");
  const uninstalled = await client.uninstall("browser", "DELETE");
  assertEqual(installed.status, "installed");
  assertEqual(uninstalled.status, "uninstalled");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/install");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/skillhub/uninstall");
  assertEqual(calls[1]?.init?.method, "DELETE");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { slug: "browser" });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { slug: "browser" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("SkillHubInstallClient checks updates updates and rolls back", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubInstallClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("check-updates")) return jsonResponse({ updates: [{ slug: "browser" }] }); return jsonResponse({ ok: true, report: { score: 99 } }); } });
  const updates = await client.checkUpdates();
  const updated = await client.update("browser");
  const rolledBack = await client.rollback("browser", "1.0.0");
  assertEqual(updates.updates.length, 1);
  assertEqual(updated.ok, true);
  assertEqual(rolledBack.ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/check-updates");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/skillhub/update");
  assertEqual(calls[2]?.url, "http://localhost:9090/api/skillhub/rollback");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { slug: "browser" });
  assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { slug: "browser", version: "1.0.0" });
});

test("SkillHubInstallClient exposes install nested gateway errors", async () => {
  const client = createSkillHubInstallClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested install failure" } }, { status: 400 }) });
  try { await client.install(""); throw new Error("expected install to reject"); } catch (error) { assert(error instanceof SkillHubInstallClientError); assertEqual(error.name, "SkillHubClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested install failure" } }); assertEqual(error.message, "nested install failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
