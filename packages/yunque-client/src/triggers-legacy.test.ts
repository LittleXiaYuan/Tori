import { createTriggersLegacyClient, TriggersLegacyClientError } from "./triggers-legacy";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("TriggersLegacyClient lists and gets legacy triggers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersLegacyClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("id=")) return jsonResponse({ id: "trg-1", name: "legacy" }); return jsonResponse({ triggers: [{ id: "trg-1", name: "legacy" }], total: 1 }); } });
  assertEqual((await client.list()).total, 1); assertEqual((await client.get("trg-1")).name, "legacy"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/triggers"); assertEqual(calls[1]?.url, "http://localhost:9090/v1/triggers?id=trg-1"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("TriggersLegacyClient creates deletes and emits legacy triggers with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createTriggersLegacyClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (String(url).includes("emit")) return jsonResponse({ status: "emitted", event: "task_completed" }); if (init?.method === "DELETE") return jsonResponse({ deleted: "trg-1" }); return jsonResponse({ id: "trg-1", name: "legacy" }, { status: 201 }); } });
  assertEqual((await client.create({ name: "legacy" })).id, "trg-1"); assertEqual((await client.emit({ event: "task_completed" })).event, "task_completed"); assertEqual((await client.delete("trg-1")).deleted, "trg-1"); assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { name: "legacy" }); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123"); assertEqual(calls[2]?.url, "http://localhost:9090/v1/triggers?id=trg-1");
});

test("TriggersLegacyClient exposes nested legacy errors", async () => {
  const client = createTriggersLegacyClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "legacy name is required" } }, { status: 400 }) });
  try { await client.create({ name: "" }); throw new Error("expected create to reject"); } catch (error) { assert(error instanceof TriggersLegacyClientError); assertEqual(error.name, "TriggersClientError"); assertEqual(error.status, 400); assertEqual(error.message, "legacy name is required"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
