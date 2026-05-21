import { createForkControlClient, ForkControlClientError } from "./fork-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
const fork = { id: "fork_1", session_id: "s1", messages: [{ role: "user", content: "你好" }], created_at: "2026-05-12T00:00:00Z", children: ["fork_2"] };

test("ForkControlClient creates and branches forks with bearer token", async () => {
  const calls: { url: string; init?: RequestInit; body?: unknown }[] = [];
  const branch = { ...fork, id: "fork_2", parent_id: "fork_1", label: "尝试另一种写法", children: [] };
  const client = createForkControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init, body: JSON.parse(String(init?.body)) }); if (String(url).includes("/branch")) return jsonResponse(branch); return jsonResponse(fork); } });
  assertEqual((await client.create({ session_id: "s1", messages: [{ role: "user", content: "你好" }] })).id, "fork_1");
  assertEqual((await client.branch({ fork_id: "fork_1", at_index: 0, label: "尝试另一种写法" })).parent_id, "fork_1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/fork");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/fork/branch");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(calls[1]?.body, { fork_id: "fork_1", at_index: 0, label: "尝试另一种写法" });
});

test("ForkControlClient removes forks with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createForkControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ deleted: true }); } });
  assertEqual((await client.remove("fork_1")).deleted, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/fork?id=fork_1");
  assertEqual(calls[0]?.init?.method, "DELETE");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ForkControlClient exposes nested control errors", async () => {
  const client = createForkControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FORK_CONTROL", message: "fork id required" } }, { status: 400 }) });
  try { await client.remove(""); throw new Error("expected remove to reject"); } catch (error) { assert(error instanceof ForkControlClientError); assertEqual(error.name, "ForkClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "FORK_CONTROL", message: "fork id required" } }); assertEqual(error.message, "fork id required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
