import { createCognisWorkflowsClient, CognisWorkflowsClientError } from "./cognis-workflows";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void { const a = JSON.stringify(actual); const e = JSON.stringify(expected); if (a !== e) throw new Error(message || `expected ${a} to deep equal ${e}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("CognisWorkflowsClient lists and runs workflows with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisWorkflowsClient({ baseUrl: "http://localhost:9090/", token: "jwt", fetch: async (url, init) => { calls.push({ url: String(url), init }); const path = String(url); if (path.endsWith("/workflows")) return jsonResponse({ workflows: ["summarize"] }); return jsonResponse({ run_id: "run-1" }); } });
  assertDeepEqual((await client.list("doc/id") as { workflows?: string[] }).workflows, ["summarize"]);
  assertEqual((await client.run("doc/id", "daily/review", { dry_run: true }) as { run_id?: string }).run_id, "run-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/workflows");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cognis/doc%2Fid/workflow/daily%2Freview");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer jwt");
  assertDeepEqual(JSON.parse(String(calls[1]?.init?.body)), { dry_run: true });
});

test("CognisWorkflowsClient supports API key auth and default run body", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCognisWorkflowsClient({ baseUrl: "http://localhost:9090", apiKey: "key", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ status: "ok" }); } });
  await client.run("doc", "nightly");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key");
  assertDeepEqual(JSON.parse(String(calls[0]?.init?.body)), {});
});

test("CognisWorkflowsClient exposes nested workflow errors", async () => {
  const client = createCognisWorkflowsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "WORKFLOW", message: "workflow failed" } }, { status: 409 }) });
  try { await client.run("doc", "nightly"); throw new Error("expected run to reject"); } catch (error) { assert(error instanceof CognisWorkflowsClientError); assertEqual(error.name, "CognisClientError"); assertEqual(error.status, 409); assertDeepEqual(error.body, { error: { code: "WORKFLOW", message: "workflow failed" } }); assertEqual(error.message, "workflow failed"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
