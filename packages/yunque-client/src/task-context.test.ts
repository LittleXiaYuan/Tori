import { createTaskContextClient, TaskContextClientError } from "./task-context";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TaskContextClient lists gaps, stats and resolves a gap", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskContextClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("stats=true")) return jsonResponse({ total: 2, unresolved: 1 }); if (String(url).includes("resolve")) return jsonResponse({ resolved: "gap-1" }); return jsonResponse([{ id: "gap-1", gap_type: "skill_missing", resolved: false }]); } });
  const gaps = await client.gaps("skill_missing"); const stats = await client.gapStats(); const resolved = await client.resolveGap("gap-1");
  assertEqual(gaps[0]?.id, "gap-1"); assertEqual(stats.total, 2); assertEqual(resolved.resolved, "gap-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/gaps?type=skill_missing"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
  assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { id: "gap-1" });
});

test("TaskContextClient reads working memory with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskContextClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ task_id: "task-1", goal: "ship planner", next_action: "resume" }); } });
  const memory = await client.workingMemory("task-1");
  assertEqual(memory.next_action, "resume"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/memory?id=task-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("TaskContextClient manages task templates", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskContextClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST" && String(url).includes("instantiate")) return jsonResponse({ id: "task-1", title: "Review" }, { status: 201 }); if (init?.method === "POST") return jsonResponse({ id: "tpl-1", name: "Review" }, { status: 201 }); if (init?.method === "DELETE") return jsonResponse({ deleted: "tpl-1" }); if (String(url).includes("id=tpl-1")) return jsonResponse({ id: "tpl-1", name: "Review" }); return jsonResponse({ templates: [{ id: "tpl-1" }], total: 1 }); } });
  const list = await client.templates(); const one = await client.template("tpl-1"); const created = await client.createTemplate({ id: "tpl-1", name: "Review" }); const task = await client.instantiateTemplate("tpl-1", { repo: "yunque" }); const deleted = await client.deleteTemplate("tpl-1");
  assertEqual(list.total, 1); assertEqual(one.id, "tpl-1"); assertEqual(created.name, "Review"); assertEqual(task.id, "task-1"); assertEqual(deleted.deleted, "tpl-1");
  assertDeepEqual(JSON.parse(String(calls[3]?.init?.body)), { template_id: "tpl-1", variables: { repo: "yunque" } });
});

test("TaskContextClient manages task threads", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTaskContextClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ status: "posted", task_id: "task-1" }); if (init?.method === "PUT") return jsonResponse({ status: "updated", task_id: "task-1", state: "paused" }); if (String(url).includes("id=task-1")) return jsonResponse({ task_id: "task-1", info: { state: "open" }, messages: [{ role: "user", content: "hi" }] }); return jsonResponse({ threads: [{ task_id: "task-1", state: "open" }], total: 1 }); } });
  const threads = await client.threads("open"); const thread = await client.thread("task-1"); const posted = await client.postThreadMessage("task-1", "hi", { channel_type: "feishu", channel_id: "chat-1" }); const updated = await client.updateThreadState("task-1", "paused");
  assertEqual(threads.total, 1); assertEqual(thread.messages[0]?.content, "hi"); assertEqual(posted.status, "posted"); assertEqual(updated.state, "paused");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/tasks/threads?state=open"); assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { task_id: "task-1", content: "hi", channel: { channel_type: "feishu", channel_id: "chat-1" } });
});

test("TaskContextClient throws TaskContextClientError with parsed body", async () => {
  const client = createTaskContextClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "template not found" }, { status: 404 }) });
  try { await client.template("missing"); throw new Error("expected template to reject"); } catch (error) { assert(error instanceof TaskContextClientError); assertEqual(error.status, 404); assertDeepEqual(error.body, { error: "template not found" }); assertEqual(error.message, "template not found"); }
  const nestedClient = createTaskContextClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "task context id is required" } }, { status: 400 }) });
  try { await nestedClient.workingMemory(""); throw new Error("expected nested memory to reject"); } catch (error) { assert(error instanceof TaskContextClientError); assertEqual(error.status, 400); assertEqual(error.message, "task context id is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
