import { createSkillHubUpdatesClient, SkillHubUpdatesClientError } from "./skillhub-updates";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillHubUpdatesClient checks and applies updates with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubUpdatesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("check-updates")) return jsonResponse({ updates: [{ slug: "browser" }] }); return jsonResponse({ ok: true, report: { changed: true } }); } });
  const updates = await client.checkUpdates(); const result = await client.update("browser");
  assertEqual(updates.updates.length, 1); assertEqual(result.ok, true); assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/check-updates"); assertEqual(calls[1]?.url, "http://localhost:9090/api/skillhub/update"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { slug: "browser" });
});

test("SkillHubUpdatesClient rolls back and lists versions with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubUpdatesClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("versions")) return jsonResponse({ versions: [{ version: "1.0.0" }] }); return jsonResponse({ ok: true }); } });
  const rollback = await client.rollback("browser", "1.0.0"); const versions = await client.versions("browser");
  assertEqual(rollback.ok, true); assertEqual(versions.versions.length, 1); assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/rollback"); assertEqual(calls[1]?.url, "http://localhost:9090/api/skillhub/versions?slug=browser"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { slug: "browser", version: "1.0.0" });
});

test("SkillHubUpdatesClient exposes nested update errors", async () => {
  const client = createSkillHubUpdatesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "slug is required" } }, { status: 400 }) });
  try { await client.update(""); throw new Error("expected update to reject"); } catch (error) { assert(error instanceof SkillHubUpdatesClientError); assertEqual(error.name, "SkillHubClientError"); assertEqual(error.status, 400); assertEqual(error.message, "slug is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
