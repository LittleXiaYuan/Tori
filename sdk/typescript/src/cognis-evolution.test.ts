import { createCognisEvolutionClient, CognisEvolutionClientError } from "./cognis-evolution";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisEvolutionClient triggers evolution and reads status with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisEvolutionClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/evolution")) return jsonResponse({ generation: 2 }); return jsonResponse({ status: "ok" }); } });
  assertEqual((await client.evolve("doc/id", { dry_run: true })).status, "ok");
  assertEqual((await client.status("doc/id") as { generation?: number }).generation, 2);
  assertEqual((await client.status() as { generation?: number }).generation, 2);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/evolve");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), { dry_run: true });
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/evolution");
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cognis/evolution");
});

test("CognisEvolutionClient supports API key auth and default evolve body", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisEvolutionClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.evolve("doc");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
});

test("CognisEvolutionClient exposes nested evolution errors", async () => {
  const client = createCognisEvolutionClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "EVOLUTION", message: "evolution failed" } }, { status: 500 }) });
  try { await client.evolve("doc"); throw new Error("expected evolve to reject"); } catch (error) { assert(error instanceof CognisEvolutionClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "EVOLUTION", message: "evolution failed" } }); assertEqual(error.message, "evolution failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
