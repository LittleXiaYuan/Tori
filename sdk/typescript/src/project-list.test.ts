import { createProjectListClient, ProjectListClientError } from "./project-list";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
const project = { id: "p1", name: "云雀", repo_path: "C:/Code/AI/云雀/yunque-agent", created_at: "2026-05-12T00:00:00Z", updated_at: "2026-05-12T00:00:00Z" };

test("ProjectListClient lists projects with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProjectListClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ projects: [project] }); } });
  const result = await client.list();
  assertEqual(result.projects[0]?.id, "p1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/projects");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ProjectListClient lists projects with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProjectListClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ projects: [] }); } });
  const result = await client.list();
  assertDeepEqual(result, { projects: [] });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/projects");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("ProjectListClient exposes project-list nested gateway errors", async () => {
  const client = createProjectListClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_GATEWAY", message: "nested project list failure" } }, { status: 502 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ProjectListClientError); assertEqual(error.name, "ProjectsClientError"); assertEqual(error.status, 502); assertDeepEqual(error.body, { error: { code: "BAD_GATEWAY", message: "nested project list failure" } }); assertEqual(error.message, "nested project list failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
