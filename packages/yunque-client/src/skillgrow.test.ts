import { createSkillGrowClient, SkillGrowClientError } from "./skillgrow";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("SkillGrowClient reads skill growth patterns through skillgrow facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSkillGrowClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ patterns: [{ pattern: "retry_then_fix", count: 2 }], count: 1 }); } });

  const result = await client.patterns();

  assertEqual(result.count, 1);
  assertEqual(result.patterns[0]?.pattern, "retry_then_fix");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/skillgrow/patterns");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SkillGrowClient exposes skillgrow-named errors", async () => {
  const client = createSkillGrowClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "skill growth unavailable" } }, { status: 503 }) });

  try {
    await client.patterns();
    throw new Error("expected patterns to reject");
  } catch (error) {
    assert(error instanceof SkillGrowClientError);
    assertEqual(error.status, 503);
    assertEqual(error.message, "skill growth unavailable");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
