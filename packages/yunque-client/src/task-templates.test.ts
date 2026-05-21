import { createTaskTemplatesClient, TaskTemplatesClientError } from "./task-templates";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskTemplatesClient lists and gets templates with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskTemplatesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("id=tpl-1")) return jsonResponse({ id: "tpl-1", name: "Review" }); return jsonResponse({ templates: [{ id: "tpl-1", name: "Review" }], total: 1 }); } });
  const list = await client.list();
  const one = await client.get("tpl-1");
  assertEqual(list.total, 1);
  assertEqual(one.name, "Review");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/templates");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/tasks/templates?id=tpl-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TaskTemplatesClient creates deletes and instantiates templates with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskTemplatesClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("instantiate")) return jsonResponse({ id: "task-1", title: "Review" }, { status: 201 }); if (init?.method === "DELETE") return jsonResponse({ deleted: "tpl-1" }); return jsonResponse({ id: "tpl-1", name: "Review" }, { status: 201 }); } });
  const created = await client.create({ id: "tpl-1", name: "Review", steps: [{ action: "review" }] });
  const task = await client.instantiate("tpl-1", { repo: "yunque" });
  const deleted = await client.delete("tpl-1");
  assertEqual(created.id, "tpl-1");
  assertEqual(task.id, "task-1");
  assertEqual(deleted.deleted, "tpl-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/templates");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "tpl-1", name: "Review", steps: [{ action: "review" }] });
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { template_id: "tpl-1", variables: { repo: "yunque" } });
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/tasks/templates?id=tpl-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
});

test("TaskTemplatesClient instantiates with empty variables by default", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskTemplatesClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ id: "task-1" }); } });
  await client.instantiate("tpl-1");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { template_id: "tpl-1", variables: {} });
});

test("TaskTemplatesClient exposes template nested gateway errors", async () => {
  const client = createTaskTemplatesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested task template failure" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof TaskTemplatesClientError); assertEqual(error.name, "TaskContextClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested task template failure" } }); assertEqual(error.message, "nested task template failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
