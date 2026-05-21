import { createSpeechClient, SpeechClientError } from "./speech";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SpeechClient synthesizes TTS audio with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSpeechClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return new Response(new Blob(["audio"]), { status: 200, headers: { "Content-Type": "audio/wav" } }); } });
  const result = await client.tts({ text: "你好", voice: "yuque", format: "wav", emotion: "happy" });
  assertEqual(result.contentType, "audio/wav"); assertEqual(await result.blob.text(), "audio");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/speech/tts"); assertEqual(calls[0]?.init?.body, JSON.stringify({ text: "你好", voice: "yuque", format: "wav", emotion: "happy" })); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SpeechClient transcribes binary audio and reads voices with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSpeechClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("/stt")) return jsonResponse({ text: "hello", emotion: { label: "calm" } }); return jsonResponse({ voices: [{ id: "v1" }], providers: ["mock"] }); } });
  const stt = await client.stt(new Uint8Array([1, 2, 3]), { language: "en", detect_emotion: true }); const voices = await client.voices();
  assertEqual(stt.text, "hello"); assertEqual(voices.providers[0], "mock"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/speech/stt?language=en&detect_emotion=true"); assert(calls[0]?.init?.body instanceof ArrayBuffer); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("SpeechClient uploads files as multipart form data", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSpeechClient({ baseUrl: "http://localhost:9090", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ filename: "note.txt", size: 4, path: "note.txt", parse: { status: "parsed", preview: "demo" } }); } });
  const uploaded = await client.upload(new Blob(["demo"], { type: "text/plain" }), "note.txt");
  assertEqual(uploaded.filename, "note.txt"); assertEqual((uploaded.parse as { status?: string }).status, "parsed"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/upload"); assert(calls[0]?.init?.body instanceof FormData); assertEqual(new Headers(calls[0]?.init?.headers).get("content-type"), null, "multipart boundary must be set by fetch");
});

test("SpeechClient builds STT stream websocket URLs", () => {
  const plain = createSpeechClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({}) });
  const secure = createSpeechClient({ baseUrl: "https://agent.example", fetch: async () => jsonResponse({}) });
  assertEqual(plain.sttStreamUrl({ language: "zh" }), "ws://localhost:9090/v1/speech/stt/stream?language=zh");
  assertEqual(secure.sttStreamUrl({ language: "en", detect_emotion: true }), "wss://agent.example/v1/speech/stt/stream?language=en&detect_emotion=true");
});

test("SpeechClient throws SpeechClientError with parsed and text bodies", async () => {
  const jsonClient = createSpeechClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: "speech not configured" }, { status: 500 }) });
  try { await jsonClient.voices(); throw new Error("expected voices to reject"); } catch (error) { assert(error instanceof SpeechClientError); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: "speech not configured" }); assertEqual(error.message, "speech not configured"); }
  const nestedClient = createSpeechClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "UNAVAILABLE", message: "nested speech not configured" } }, { status: 500 }) });
  try { await nestedClient.voices(); throw new Error("expected nested voices to reject"); } catch (error) { assert(error instanceof SpeechClientError); assertEqual(error.status, 500); assertEqual(error.message, "nested speech not configured"); }
  const textClient = createSpeechClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("method not allowed", { status: 405 }) });
  try { await textClient.tts({ text: "x" }); throw new Error("expected tts to reject"); } catch (error) { assert(error instanceof SpeechClientError); assertEqual(error.status, 405); assertEqual(error.body, "method not allowed"); assertEqual(error.message, "method not allowed"); }
});
let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
