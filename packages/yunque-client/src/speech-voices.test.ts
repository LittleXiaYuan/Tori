import { createSpeechVoicesClient, SpeechVoicesClientError } from "./speech-voices";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SpeechVoicesClient lists voices with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSpeechVoicesClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ voices: [{ id: "yuque" }], providers: ["local"] }); } });
  const result = await client.voices();
  assertDeepEqual(result.voices, [{ id: "yuque" }]);
  assertDeepEqual(result.providers, ["local"]);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/speech/voices");
  assertEqual(calls[0]?.init?.method, "GET");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SpeechVoicesClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSpeechVoicesClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ voices: [], providers: [] }); } });
  assertDeepEqual((await client.voices()).voices, []);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("SpeechVoicesClient exposes nested voices errors", async () => {
  const client = createSpeechVoicesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SPEECH_VOICES", message: "voices failed" } }, { status: 503 }) });
  try { await client.voices(); throw new Error("expected voices to reject"); } catch (error) { assert(error instanceof SpeechVoicesClientError); assertEqual(error.name, "SpeechClientError"); assertEqual(error.status, 503); assertDeepEqual(error.body, { error: { code: "SPEECH_VOICES", message: "voices failed" } }); assertEqual(error.message, "voices failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
