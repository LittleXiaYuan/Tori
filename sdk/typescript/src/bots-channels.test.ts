import { createBotsChannelsClient, BotsChannelsClientError } from "./bots-channels";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("BotsChannelsClient reads channel groups with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsChannelsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ groups: [{ id: "g1", channel_type: "telegram" }], count: 1 }); } });
  const groups = await client.groups("telegram");
  assertEqual(groups.groups[0]?.id, "g1"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/channels/groups?type=telegram"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
});

test("BotsChannelsClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createBotsChannelsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ groups: [], count: 0 }); } });
  await client.groups();
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/channels/groups"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("BotsChannelsClient exposes nested channel errors", async () => {
  const client = createBotsChannelsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_GATEWAY", message: "nested channel sync failed" } }, { status: 502 }) });
  try { await client.groups("telegram"); throw new Error("expected groups to reject"); } catch (error) { assert(error instanceof BotsChannelsClientError); assertEqual(error.name, "BotsClientError"); assertEqual(error.status, 502); assertDeepEqual(error.body, { error: { code: "BAD_GATEWAY", message: "nested channel sync failed" } }); assertEqual(error.message, "nested channel sync failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
