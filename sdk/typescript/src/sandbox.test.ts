import { createSandboxClient, SandboxClientError } from "./sandbox";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SandboxClient executes sandbox commands with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSandboxClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ stdout: "ok", exit_code: 0 }); } });
  const result = await client.exec({ command: "python", args: ["-V"] });
  assertEqual(result.stdout, "ok"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/sandbox/exec"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { command: "python", args: ["-V"] });
});

test("SandboxClient probes cloud and desktop readiness with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSandboxClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ key_source: "tori_oauth_bound", cloud_runner_ready: true, desktop_running: false }); } });
  const result = await client.probe();
  assertEqual(result.key_source, "tori_oauth_bound"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/sandbox/probe"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("SandboxClient manages desktop sandbox lifecycle", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSandboxClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("status")) return jsonResponse({ ok: true, running: true, alive: true, sandbox: { id: "desk-1" } }); if (String(url).includes("destroy")) return jsonResponse({ ok: true, message: "desktop sandbox destroyed" }); return jsonResponse({ ok: true, sandbox: { id: "desk-1" } }); } });
  const created = await client.createDesktop(); const status = await client.desktopStatus(); const destroyed = await client.destroyDesktop("DELETE");
  assertEqual(created.ok, true); assertEqual(status.running, true); assertEqual(destroyed.message, "desktop sandbox destroyed"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(calls[2]?.init?.method, "DELETE");
});

test("SandboxClient throws SandboxClientError with parsed and text bodies", async () => {
  const jsonClient = createSandboxClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "command is required" }, { status: 400 }) });
  try { await jsonClient.exec({ command: "" }); throw new Error("expected exec to reject"); } catch (error) { assert(error instanceof SandboxClientError); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: "command is required" }); assertEqual(error.message, "command is required"); }
  const textClient = createSandboxClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("POST only", { status: 405 }) });
  try { await textClient.createDesktop(); throw new Error("expected createDesktop to reject"); } catch (error) { assert(error instanceof SandboxClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST only"); assertEqual(error.message, "POST only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
