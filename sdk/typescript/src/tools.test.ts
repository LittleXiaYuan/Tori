import { createToolsClient, ToolsClientError } from "./tools";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ToolsClient executes commands with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToolsClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ output: "ok", exit_code: 0, state: "exited" }); } });
  const result = await client.exec({ Command: "echo ok", Cwd: "work", TimeoutMs: 1000, Env: ["A=B"] });
  assertEqual(result.output, "ok"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/tools/exec"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { Command: "echo ok", Cwd: "work", TimeoutMs: 1000, Env: ["A=B"] });
});

test("ToolsClient lists and polls sessions with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToolsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("poll")) return jsonResponse({ lines: ["hello"], state: "running" }); return jsonResponse({ sessions: [{ id: "s1", command: "npm test", state: "running", exit_code: 0 }] }); } });
  const list = await client.list(); const poll = await client.poll("s1");
  assertEqual(list.sessions[0]?.id, "s1"); assertEqual(poll.lines?.[0], "hello"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/tools/poll?id=s1"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ToolsClient kills background sessions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createToolsClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ killed: "s1" }); } });
  const result = await client.kill("s1");
  assertEqual(result.killed, "s1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/tools/kill?id=s1"); assertEqual(calls[0]?.init?.method, "POST");
});

test("ToolsClient throws ToolsClientError with parsed and text bodies", async () => {
  const jsonClient = createToolsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "Command blocked by shell policy", risk: "high" }, { status: 403 }) });
  try { await jsonClient.exec({ Command: "rm -rf /" }); throw new Error("expected exec to reject"); } catch (error) { assert(error instanceof ToolsClientError); assertEqual(error.status, 403); assertDeepEqual(error.body, { error: "Command blocked by shell policy", risk: "high" }); assertEqual(error.message, "Command blocked by shell policy"); }
  const nestedClient = createToolsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "FORBIDDEN", message: "nested command blocked" }, risk: "high" }, { status: 403 }) });
  try { await nestedClient.exec({ Command: "rm -rf /" }); throw new Error("expected nested exec to reject"); } catch (error) { assert(error instanceof ToolsClientError); assertEqual(error.status, 403); assertEqual(error.message, "nested command blocked"); }
  const textClient = createToolsClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("session id required", { status: 400 }) });
  try { await textClient.poll(""); throw new Error("expected poll to reject"); } catch (error) { assert(error instanceof ToolsClientError); assertEqual(error.status, 400); assertEqual(error.body, "session id required"); assertEqual(error.message, "session id required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
