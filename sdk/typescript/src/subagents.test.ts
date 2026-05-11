import { createSubagentsClient, SubagentsClientError } from "./subagents";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SubagentsClient lists subagents with parent filter and bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSubagentsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ subagents: [{ id: "sa-1", name: "reviewer", parent_id: "task-1" }] }); } });
  const result = await client.list("task-1");
  assertEqual(result.subagents[0]?.id, "sa-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/subagent?parent_id=task-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SubagentsClient gets and spawns subagents with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSubagentsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "POST") return jsonResponse({ id: "sa-2", name: "planner", skills: ["plan"] }); return jsonResponse({ id: "sa-1", name: "reviewer" }); } });
  const one = await client.get("sa-1"); const spawned = await client.spawn({ parent_id: "task-1", name: "planner", description: "计划拆解", skills: ["plan"] });
  assertEqual(one.name, "reviewer"); assertEqual(spawned.skills?.[0], "plan"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { parent_id: "task-1", name: "planner", description: "计划拆解", skills: ["plan"] });
});

test("SubagentsClient appends messages and destroys subagents", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSubagentsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("message")) return jsonResponse({ ok: true }); return jsonResponse({ deleted: true }); } });
  const appended = await client.appendMessages("sa-1", [{ role: "user", content: "继续" }]); const destroyed = await client.destroy("sa-1");
  assertEqual(appended.ok, true); assertEqual(destroyed.deleted, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/subagent/message"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { id: "sa-1", messages: [{ role: "user", content: "继续" }] }); assertEqual(calls[1]?.init?.method, "DELETE"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/subagent?id=sa-1");
});

test("SubagentsClient throws SubagentsClientError with parsed and text bodies", async () => {
  const jsonClient = createSubagentsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "subagent not found" }, { status: 400 }) });
  try { await jsonClient.get("missing"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof SubagentsClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "subagent not found" }); assertEqual(error.message, "subagent not found"); }
  const textClient = createSubagentsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST required", { status: 405 }) });
  try { await textClient.appendMessages("sa-1", []); throw new Error("expected appendMessages to reject"); } catch (error) { assert(error instanceof SubagentsClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST required"); assertEqual(error.message, "POST required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
