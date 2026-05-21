import { createSetupInstallClient, SetupInstallClientError } from "./setup-install";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }
function sseResponse(chunks: string[]): Response { const stream = new ReadableStream<Uint8Array>({ start(controller) { const encoder = new TextEncoder(); for (const chunk of chunks) controller.enqueue(encoder.encode(chunk)); controller.close(); } }); return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } }); }

test("SetupInstallClient installs components with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupInstallClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ success: true, message: "installed" }); } });
  const result = await client.install("python_office");
  assertEqual(result.success, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/install-component");
  assertEqual(calls[0]?.init?.method, "POST");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { component_id: "python_office" });
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("SetupInstallClient streams install progress with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupInstallClient({ baseUrl: "http://localhost:9090", apiKey: "ya", fetch: async (url, init) => { calls.push({ url: String(url), init }); return sseResponse(['data: {"stage":"download","progress":50}\n\n', 'data: {"stage":"done","progress":100}\n\n']); } });
  const events = [];
  for await (const event of client.stream("python_office")) events.push(event);
  assertEqual(events.length, 2);
  assertDeepEqual(events[0], { stage: "download", progress: 50 });
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/install-component");
  assertEqual(new Headers(calls[0]?.init?.headers).get("accept"), "text/event-stream");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { component_id: "python_office" });
});

test("SetupInstallClient exposes nested install errors", async () => {
  const client = createSetupInstallClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested setup install failure" } }, { status: 400 }) });
  try { await client.install(""); throw new Error("expected install to reject"); } catch (error) { assert(error instanceof SetupInstallClientError); assertEqual(error.name, "SetupClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested setup install failure" } }); assertEqual(error.message, "nested setup install failure"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
