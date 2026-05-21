import { createIdentityClient, IdentityClientError } from "./identity";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("IdentityClient resolves identities through identity facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIdentityClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ unified_id: "wechat:u1", display_name: "小云" }); } });

  const result = await client.resolve({ channel: "wechat", user_id: "u1", display_name: "小云" });

  assertEqual(result.unified_id, "wechat:u1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/identity/resolve");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ channel: "wechat", user_id: "u1", display_name: "小云" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("IdentityClient lists identity profiles without generated SDK", async () => {
  const client = createIdentityClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { assertEqual(String(url), "http://localhost:9090/v1/identity/profiles"); assertEqual(new Headers(init?.headers).get("x-api-key"), "key-123"); return jsonResponse({ profiles: [{ unified_id: "qq:u2" }] }); } });

  const result = await client.profiles();

  assertEqual(result.profiles[0]?.unified_id, "qq:u2");
});

test("IdentityClient exposes identity-named errors", async () => {
  const client = createIdentityClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "identity disabled" } }, { status: 403 }) });

  try {
    await client.profiles();
    throw new Error("expected profiles to reject");
  } catch (error) {
    assert(error instanceof IdentityClientError);
    assertEqual(error.status, 403);
    assertEqual(error.message, "identity disabled");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
