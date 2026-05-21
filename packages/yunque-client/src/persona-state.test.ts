import { createPersonaStateClient, PersonaStateClientError } from "./persona-state";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PersonaStateClient reads persona state with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaStateClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ identity: "Tori", soul: "warm", skills: [{ name: "plan" }] }); } });
  const result = await client.get();
  assertEqual(result.identity, "Tori");
  assertEqual(result.skills?.[0]?.name, "plan");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PersonaStateClient updates identity and soul with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaStateClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.update({ identity: "云雀", soul: "careful" })).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ identity: "云雀", soul: "careful" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("PersonaStateClient exposes nested persona state errors", async () => {
  const client = createPersonaStateClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "PERSONA_STATE", message: "state failed" } }, { status: 409 }) });
  try { await client.update({ identity: "" }); throw new Error("expected update to reject"); } catch (error) { assert(error instanceof PersonaStateClientError); assertEqual(error.name, "PersonaClientError"); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: { code: "PERSONA_STATE", message: "state failed" } }); assertEqual(error.message, "state failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
