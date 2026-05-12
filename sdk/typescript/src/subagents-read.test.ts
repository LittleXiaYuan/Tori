import { createSubagentsReadClient, SubagentsReadClientError } from "./subagents-read";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SubagentsReadClient lists subagents with parent filter and bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSubagentsReadClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ subagents: [{ id: "sa-1", name: "reviewer", parent_id: "task-1" }] }); } });
  const result = await client.list("task-1");
  assertEqual(result.subagents[0]?.id, "sa-1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/subagent?parent_id=task-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SubagentsReadClient gets subagents with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSubagentsReadClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ id: "sa-1", name: "reviewer" }); } });
  const one = await client.get("sa-1");
  assertEqual(one.name, "reviewer"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/subagent?id=sa-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("SubagentsReadClient exposes nested read errors", async () => {
  const client = createSubagentsReadClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "subagent id is required" } }, { status: 400 }) });
  try { await client.get(""); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof SubagentsReadClientError); assertEqual(error.name, "SubagentsClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "subagent id is required" } }); assertEqual(error.message, "subagent id is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
