import { createDiscoveryIdentityClient, DiscoveryIdentityClientError } from "./discovery-identity";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("DiscoveryIdentityClient resolves identity with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createDiscoveryIdentityClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ unified_id: "u1", display_name: "小羽", channels: { feishu: "42" } }); } });
  const profile = await client.resolveIdentity({ channel: "feishu", user_id: "42", display_name: "小羽" });
  assertEqual(profile.unified_id, "u1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/identity/resolve"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { channel: "feishu", user_id: "42", display_name: "小羽" });
});

test("DiscoveryIdentityClient lists identity profiles", async () => {
  const client = createDiscoveryIdentityClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ profiles: [{ unified_id: "u1", display_name: "云雀" }] }) });
  const profiles = await client.identityProfiles();
  assertEqual(profiles.profiles[0]?.display_name, "云雀");
});

test("DiscoveryIdentityClient exposes nested identity errors", async () => {
  const client = createDiscoveryIdentityClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "identity channel is required" } }, { status: 400 }) });
  try { await client.resolveIdentity({ channel: "", user_id: "42" }); throw new Error("expected resolveIdentity to reject"); } catch (error) { assert(error instanceof DiscoveryIdentityClientError); assertEqual(error.name, "DiscoveryClientError"); assertEqual(error.status, 400); assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "identity channel is required" } }); assertEqual(error.message, "identity channel is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
