import { createSkillHubCatalogClient, SkillHubCatalogClientError } from "./skillhub-catalog";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillHubCatalogClient searches catalog with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubCatalogClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ results: [{ name: "browser", source: "clawhub", installed: false }], count: 1 }); } });
  const result = await client.search({ q: "browser", limit: 5, source: "clawhub" });
  assertEqual(result.count, 1);
  assertEqual(result.results[0]?.name, "browser");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/search?q=browser&limit=5&source=clawhub");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SkillHubCatalogClient reads trending and detail with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillHubCatalogClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("detail")) return jsonResponse({ slug: "browser", name: "Browser", tags: ["automation"] }); return jsonResponse({ skills: [{ name: "browser", source: "local" }], count: 1, next_cursor: "n2" }); } });
  const trending = await client.trending({ limit: 3, cursor: "n1" });
  const detail = await client.detail("browser");
  assertEqual(trending.next_cursor, "n2");
  assertEqual(detail.slug, "browser");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/skillhub/trending?limit=3&cursor=n1");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/skillhub/detail?slug=browser");
  assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "ya");
});

test("SkillHubCatalogClient exposes catalog nested gateway errors", async () => {
  const client = createSkillHubCatalogClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested catalog failure" } }, { status: 400 }) });
  try { await client.detail(""); throw new Error("expected detail to reject"); } catch (error) { assert(error instanceof SkillHubCatalogClientError); assertEqual(error.name, "SkillHubClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested catalog failure" } }); assertEqual(error.message, "nested catalog failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
