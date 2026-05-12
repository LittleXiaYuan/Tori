import { createAdminDesktopClient, AdminDesktopClientError } from "./admin-desktop";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AdminDesktopClient controls console and autostart with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAdminDesktopClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("console")) return jsonResponse({ console_hidden: init?.method === "POST" }); return jsonResponse({ autostart_enabled: init?.method === "POST" }); } });
  assertEqual((await client.consoleStatus()).console_hidden, false);
  assertEqual((await client.toggleConsole()).console_hidden, true);
  assertEqual((await client.autostartStatus()).autostart_enabled, false);
  assertEqual((await client.toggleAutostart()).autostart_enabled, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/desktop/console");
  assertEqual(calls[1]?.init?.method, "POST");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("AdminDesktopClient exposes nested desktop errors", async () => {
  const client = createAdminDesktopClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "DESKTOP", message: "desktop unavailable" } }, { status: 503 }) });
  try { await client.consoleStatus(); throw new Error("expected consoleStatus to reject"); } catch (error) { assert(error instanceof AdminDesktopClientError); assertEqual(error.name, "AdminClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "DESKTOP", message: "desktop unavailable" } }); assertEqual(error.message, "desktop unavailable"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
