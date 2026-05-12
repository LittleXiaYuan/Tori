import { createProviderSessionClient, ProviderSessionClientError } from "./provider-session";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ProviderSessionClient sets and clears session provider with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderSessionClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ ok: true, provider_id: "p1" }); } });
  const set = await client.setSessionProvider({ session_id: "s1", provider_id: "p1" }); const cleared = await client.clearSessionProvider("s1");
  assertEqual(set.ok, true); assertEqual(cleared.ok, true); assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/session"); assertEqual(calls[1]?.url, "http://localhost:9090/api/providers/session"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { session_id: "s1", provider_id: "p1" }); assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { session_id: "s1", provider_id: "" });
});

test("ProviderSessionClient exposes nested session errors", async () => {
  const client = createProviderSessionClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "session_id is required" } }, { status: 400 }) });
  try { await client.clearSessionProvider(""); throw new Error("expected clearSessionProvider to reject"); } catch (error) { assert(error instanceof ProviderSessionClientError); assertEqual(error.name, "ProvidersClientError"); assertEqual(error.status, 400); assertEqual(error.message, "session_id is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
