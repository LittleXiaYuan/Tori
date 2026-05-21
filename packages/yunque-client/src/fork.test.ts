import { createForkClient, ForkClientError } from "./fork";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
const fork = { id: "fork_1", session_id: "s1", messages: [{ role: "user", content: "你好", timestamp: "2026-05-12T00:00:00Z" }], created_at: "2026-05-12T00:00:00Z", children: ["fork_2"] };

test("ForkClient reads root and specific fork with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createForkClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse(fork); } });
  const root = await client.root("s1"); const got = await client.get("fork_1");
  assertEqual((root as typeof fork).id, "fork_1"); assertEqual(got.session_id, "s1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/fork?session_id=s1"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/fork?id=fork_1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ForkClient creates and branches forks with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const branch = { ...fork, id: "fork_2", parent_id: "fork_1", label: "尝试另一种写法", children: [] };
  const client = createForkClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/branch")) return jsonResponse(branch); return jsonResponse(fork); } });
  const created = await client.create({ session_id: "s1", messages: [{ role: "user", content: "你好" }] }); const branched = await client.branch({ fork_id: "fork_1", at_index: 0, label: "尝试另一种写法" });
  assertEqual(created.id, "fork_1"); assertEqual(branched.parent_id, "fork_1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/fork"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/fork/branch"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { session_id: "s1", messages: [{ role: "user", content: "你好" }] }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { fork_id: "fork_1", at_index: 0, label: "尝试另一种写法" });
});

test("ForkClient lists branches and removes forks", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createForkClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ deleted: true }); return jsonResponse({ forks: [fork] }); } });
  const list = await client.list("s1"); const removed = await client.remove("fork_1");
  assertEqual(list.forks[0]?.id, "fork_1"); assertEqual(removed.deleted, true); assertEqual(calls[0]?.url, "http://localhost:9090/v1/fork/list?session_id=s1"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/fork?id=fork_1"); assertEqual(calls[1]?.init?.method, "DELETE");
});

test("ForkClient preserves empty root response", async () => {
  const client = createForkClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ fork: null }) });
  const root = await client.root("empty");
  assertDeepEqual(root, { fork: null });
});

test("ForkClient throws ForkClientError with parsed and text bodies", async () => {
  const jsonClient = createForkClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "session_id required" }, { status: 400 }) });
  try { await jsonClient.list(""); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ForkClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "session_id required" }); assertEqual(error.message, "session_id required"); }
  const nestedClient = createForkClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "fork session id is required" } }, { status: 400 }) });
  try { await nestedClient.list(""); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ForkClientError); assertEqual(error.status, 400); assertEqual(error.message, "fork session id is required"); }
  const textClient = createForkClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.get("fork_1"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof ForkClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
