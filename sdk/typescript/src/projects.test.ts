import { createProjectsClient, ProjectsClientError } from "./projects";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ProjectsClient lists and creates projects with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const project = { id: "p1", name: "云雀", repo_path: "C:/Code/AI/云雀/yunque-agent", created_at: "2026-05-12T00:00:00Z", updated_at: "2026-05-12T00:00:00Z" };
  const client = createProjectsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse(project, { status: 201 }); return jsonResponse({ projects: [project] }); } });
  const list = await client.list(); const created = await client.create({ name: "云雀", repo_path: "C:/Code/AI/云雀/yunque-agent", default_caps: ["read", "write"], meta: { owner: "yunque" } });
  assertEqual(list.projects[0]?.id, "p1"); assertEqual(created.repo_path, "C:/Code/AI/云雀/yunque-agent"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/projects"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { name: "云雀", repo_path: "C:/Code/AI/云雀/yunque-agent", default_caps: ["read", "write"], meta: { owner: "yunque" } });
});

test("ProjectsClient reads and updates project detail with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProjectsClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ id: "p1", name: "云雀+", repo_path: "C:/repo", repo_url: "https://example.invalid/yunque", description: "Agent", default_caps: ["read"], meta: { stage: "dev" }, created_at: "2026-05-12T00:00:00Z", updated_at: "2026-05-12T00:01:00Z" }); } });
  const detail = await client.detail("p1"); const updated = await client.update("p1", { name: "云雀+", repo_url: "https://example.invalid/yunque", description: "Agent", default_caps: ["read"], meta: { stage: "dev" } });
  assertEqual(detail.id, "p1"); assertEqual(updated.name, "云雀+"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/projects/detail?id=p1"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/projects/detail?id=p1"); assertEqual(calls[1]?.init?.method, "PUT"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { name: "云雀+", repo_url: "https://example.invalid/yunque", description: "Agent", default_caps: ["read"], meta: { stage: "dev" } });
});

test("ProjectsClient removes projects with body id", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProjectsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "deleted" }); } });
  const removed = await client.remove("p1");
  assertEqual(removed.status, "deleted"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/projects/remove"); assertEqual(calls[0]?.init?.method, "POST"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "p1" });
});

test("ProjectsClient throws ProjectsClientError with parsed and text bodies", async () => {
  const jsonClient = createProjectsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "repo_path is required" }, { status: 400 }) });
  try { await jsonClient.create({ name: "bad", repo_path: "" }); throw new Error("expected create to reject"); } catch (error) { assert(error instanceof ProjectsClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "repo_path is required" }); assertEqual(error.message, "repo_path is required"); }
  const textClient = createProjectsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ProjectsClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
