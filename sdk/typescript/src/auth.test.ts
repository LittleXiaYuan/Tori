import { createAuthClient, AuthClientError } from "./auth";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("AuthClient exchanges API key for a user token by default", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuthClient({ baseUrl: "http://localhost:9090/", apiKey: "ya_test", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ token: "jwt-user", type: "Bearer" }); } });
  const result = await client.generateToken();
  assertEqual(result.token, "jwt-user"); assertEqual(result.type, "Bearer"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/token"); assertEqual(calls[0]?.init?.method, "POST"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "ya_test"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
});

test("AuthClient sends requested viewer role and supports bearer-style API key exchange", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuthClient({ baseUrl: "http://localhost:9090", token: "ya_bearer", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ token: "jwt-viewer", type: "Bearer" }); } });
  const result = await client.generateToken({ role: "viewer" });
  assertEqual(result.token, "jwt-viewer"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer ya_bearer"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { role: "viewer" });
});

test("AuthClient throws AuthClientError with parsed and text bodies", async () => {
  const jsonClient = createAuthClient({ baseUrl: "http://localhost:9090", apiKey: "bad", fetch: async () => jsonResponse({ error: "invalid api key" }, { status: 401 }) });
  try { await jsonClient.generateToken(); throw new Error("expected generateToken to reject"); } catch (error) { assert(error instanceof AuthClientError); assertEqual(error.status, 401); assertDeepEqual(error.body, { error: "invalid api key" }); assertEqual(error.message, "invalid api key"); }
  const textClient = createAuthClient({ baseUrl: "http://localhost:9090", apiKey: "bad", fetch: async () => new Response("POST only", { status: 405 }) });
  try { await textClient.generateToken(); throw new Error("expected text generateToken to reject"); } catch (error) { assert(error instanceof AuthClientError); assertEqual(error.status, 405); assertEqual(error.body, "POST only"); assertEqual(error.message, "POST only"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
