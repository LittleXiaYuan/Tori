import { createCognisExperienceClient, CognisExperienceClientError } from "./cognis-experience";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisExperienceClient reads records and confirms patterns with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisExperienceClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/experience")) return jsonResponse({ enabled: true, summary: { top_tools: [{ tool: "search" }] } }); return jsonResponse({ status: "ok" }); } });
  const experience = await client.get("doc/id");
  const recorded = await client.record("doc/id", { type: "fact", data: { fact: "云雀会复用经验", source: "test" } });
  const confirmed = await client.confirmPattern("doc/id", "pat/1");
  assertEqual(experience.enabled, true);
  assertEqual(experience.summary?.top_tools?.[0]?.tool, "search");
  assertEqual(recorded.status, "ok");
  assertEqual(confirmed.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/experience");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { type: "fact", data: { fact: "云雀会复用经验", source: "test" } });
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/experience/patterns/pat%2F1/confirm");
});

test("CognisExperienceClient supports API key auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisExperienceClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ enabled: true }); } });
  await client.get("doc");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
});

test("CognisExperienceClient exposes nested experience errors", async () => {
  const client = createCognisExperienceClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "EXPERIENCE", message: "experience store failed" } }, { status: 500 }) });
  try { await client.get("doc"); throw new Error("expected get to reject"); } catch (error) { assert(error instanceof CognisExperienceClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 500); assertDeepEqual(error.body, { error: { code: "EXPERIENCE", message: "experience store failed" } }); assertEqual(error.message, "experience store failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
