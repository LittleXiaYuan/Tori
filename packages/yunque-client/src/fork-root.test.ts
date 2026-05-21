import { createForkRootClient, ForkRootClientError } from "./fork-root";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
const fork = { id: "fork_1", session_id: "s1", messages: [{ role: "user", content: "你好", timestamp: "2026-05-12T00:00:00Z" }], created_at: "2026-05-12T00:00:00Z", children: ["fork_2"] };

test("ForkRootClient reads root and specific fork with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createForkRootClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse(fork); } });
  const root = await client.root("s1");
  const got = await client.get("fork_1");
  assertEqual((root as typeof fork).id, "fork_1");
  assertEqual(got.session_id, "s1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/fork?session_id=s1");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/fork?id=fork_1");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("ForkRootClient preserves empty root and supports API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createForkRootClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ fork: null }); } });
  assertDeepEqual(await client.root("empty session/二"), { fork: null });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/fork?session_id=empty+session%2F%E4%BA%8C");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("ForkRootClient exposes fork-root nested gateway errors", async () => {
  const client = createForkRootClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FORK_ROOT", message: "nested fork root failure" } }, { status: 400 }) });
  try { await client.root(""); throw new Error("expected root to reject"); } catch (error) { assert(error instanceof ForkRootClientError); assertEqual(error.name, "ForkClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "FORK_ROOT", message: "nested fork root failure" } }); assertEqual(error.message, "nested fork root failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
