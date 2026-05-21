import { createProjectWriteClient, ProjectWriteClientError } from "./project-write";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
const project = { id: "p1", name: "云雀", repo_path: "C:/Code/AI/云雀/yunque-agent", created_at: "2026-05-12T00:00:00Z", updated_at: "2026-05-12T00:00:00Z" };

test("ProjectWriteClient creates projects with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProjectWriteClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse(project, { status: 201 }); } });
  const result = await client.create({ name: "云雀", repo_path: "C:/Code/AI/云雀/yunque-agent", default_caps: ["read", "write"], meta: { track: "sdk" } });
  assertEqual(result.id, "p1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/projects");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "云雀", repo_path: "C:/Code/AI/云雀/yunque-agent", default_caps: ["read", "write"], meta: { track: "sdk" } });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ProjectWriteClient updates and removes projects with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProjectWriteClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/remove")) return jsonResponse({ status: "deleted" }); return jsonResponse({ ...project, description: "Agent" }); } });
  const updated = await client.update("p1", { description: "Agent" });
  const removed = await client.remove("p1");
  assertEqual(updated.description, "Agent");
  assertEqual(removed.status, "deleted");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/projects/detail?id=p1");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { description: "Agent" });
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/projects/remove");
  assertEqual(calls[1]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { id: "p1" });
  assertEqual(new Headers(calls[1]?.init?.headers).get("x-api-key"), "ya");
});

test("ProjectWriteClient exposes project-write nested gateway errors", async () => {
  const client = createProjectWriteClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested project write failure" } }, { status: 400 }) });
  try { await client.create({ name: "", repo_path: "" }); throw new Error("expected create to reject"); } catch (error) { assert(error instanceof ProjectWriteClientError); assertEqual(error.name, "ProjectsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested project write failure" } }); assertEqual(error.message, "nested project write failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
