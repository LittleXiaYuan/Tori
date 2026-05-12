import { createSpeechSTTClient, SpeechSTTClientError } from "./speech-stt";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SpeechSTTClient transcribes audio with bearer token and query options", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const audio = new Blob(["voice"], { type: "audio/wav" });
  const client = createSpeechSTTClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ text: "你好", emotion: { label: "calm" } }); } });
  const result = await client.stt(audio, { language: "zh-CN", detect_emotion: true });
  assertEqual(result.text, "你好");
  assertDeepEqual(result.emotion, { label: "calm" });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/speech/stt?language=zh-CN&detect_emotion=true");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, audio);
  assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), "audio/wav");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SpeechSTTClient supports API key auth and STT stream URLs", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const bytes = new Uint8Array([1, 2, 3]);
  const client = createSpeechSTTClient({ baseUrl: "https://agent.example", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ text: "hi" }); } });
  assertEqual((await client.stt(bytes)).text, "hi");
  assert(calls[0]?.init?.body instanceof ArrayBuffer);
  assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), "application/octet-stream");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertEqual(client.sttStreamUrl({ language: "en", detect_emotion: false }), "wss://agent.example/v1/speech/stt/stream?language=en&detect_emotion=false");
});

test("SpeechSTTClient exposes nested stt errors", async () => {
  const client = createSpeechSTTClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "SPEECH_STT", message: "stt failed" } }, { status: 422 }) });
  try { await client.stt(new Blob(["x"])); throw new Error("expected stt to reject"); } catch (error) { assert(error instanceof SpeechSTTClientError); assertEqual(error.name, "SpeechClientError"); assertEqual(error.status, 422); assertDeepEqual(error.body, { error: { code: "SPEECH_STT", message: "stt failed" } }); assertEqual(error.message, "stt failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
