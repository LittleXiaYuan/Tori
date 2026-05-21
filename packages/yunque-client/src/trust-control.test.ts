import { createTrustControlClient, TrustControlClientError } from "./trust-control";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TrustControlClient reads scores with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTrustControlClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ scores: { shell: { score: 80 } }, count: 1 }); } });
  assertEqual((await client.scores()).count, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/trust/scores");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("TrustControlClient resets, grants and grants all with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTrustControlClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).endsWith("/reset")) return jsonResponse({ status: "reset", slug: "shell" }); return jsonResponse({ status: "granted", slug: "shell", upgraded: 3 }); } });
  assertEqual((await client.reset({ slug: "shell" })).status, "reset");
  assertEqual((await client.grant({ slug: "shell" })).status, "granted");
  assertEqual((await client.grantAll()).upgraded, 3);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/trust/reset");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/trust/grant");
  assertEqual(calls[2]?.url, "http://localhost:9090/api/trust/grant");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(JSON.parse(String(calls[2]?.init?.body)), { slug: "*" });
});

test("TrustControlClient exposes nested control errors", async () => {
  const client = createTrustControlClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "TRUST_CONTROL", message: "control failed" } }, { status: 400 }) });
  try { await client.reset({ slug: "shell" }); throw new Error("expected reset to reject"); } catch (error) { assert(error instanceof TrustControlClientError); assertEqual(error.name, "TrustClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "TRUST_CONTROL", message: "control failed" } }); assertEqual(error.message, "control failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
