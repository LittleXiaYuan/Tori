import { createPersonaPresetsClient, PersonaPresetsClientError } from "./persona-presets";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("PersonaPresetsClient lists and switches presets with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaPresetsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "GET") return jsonResponse({ presets: [{ id: "default", name: "Default" }], active: "default" }); return jsonResponse({ status: "ok", active: "studio" }); } });
  const presets = await client.presets();
  const switched = await client.switchPreset({ id: "studio" });
  assertEqual(presets.active, "default");
  assertEqual(switched.active, "studio");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/presets");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(calls[1]?.init?.method, "POST");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ id: "studio" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("PersonaPresetsClient manages custom presets and features with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createPersonaPresetsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/custom") && init?.method === "POST") return jsonResponse({ status: "ok", id: "studio" }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.addCustomPreset({ id: "studio", name: "Studio", features: { emotion: true } })).id, "studio");
  assertEqual((await client.updatePresetFeatures({ id: "studio", features: { sticker: false } })).status, "ok");
  assertEqual((await client.deleteCustomPreset({ id: "studio" })).status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/persona/presets/custom");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "studio", name: "Studio", features: { emotion: true } }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/persona/presets/features");
  assertEqual(calls[1]?.init?.method, "PUT");
  assertEqual(calls[2]?.init?.method, "DELETE");
  assertEqual(new Headers(calls[2]?.init?.headers).get("x-api-key"), "key");
});

test("PersonaPresetsClient exposes nested preset errors", async () => {
  const client = createPersonaPresetsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "PERSONA_PRESET", message: "preset failed" } }, { status: 400 }) });
  try { await client.switchPreset({ id: "" }); throw new Error("expected switchPreset to reject"); } catch (error) { assert(error instanceof PersonaPresetsClientError); assertEqual(error.name, "PersonaClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "PERSONA_PRESET", message: "preset failed" } }); assertEqual(error.message, "preset failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
