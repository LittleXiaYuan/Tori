import { createReflectClient, ReflectClientError } from "./reflect";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ReflectClient reads filtered experiences without importing generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReflectClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ experiences: [{ id: "e1", tags: ["quality:9"] }], total: 1 }); } });

  const result = await client.experiences({ source: "task", tag: "quality:9", limit: 1 });

  assertEqual(result.total, 1);
  assertEqual(result.experiences[0]?.tags?.[0], "quality:9");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/reflect/experiences?source=task&tag=quality%3A9&limit=1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ReflectClient reads scoped stats and strategies", async () => {
  const calls: string[] = [];
  const client = createReflectClient({ baseUrl: "http://localhost:9090", fetch: async (url) => { calls.push(String(url)); if (String(url).includes("stats=true")) return jsonResponse({ total: 2, by_outcome: { success: 2 } }); return jsonResponse({ strategies: "- 推荐: Keep high-quality strategies" }); } });

  const stats = await client.experienceStats({ tag: "quality:9" });
  const strategies = await client.strategies({ tag: "quality:9", limit: 3 });

  assertEqual(stats.by_outcome?.success, 2);
  assert(strategies.strategies.includes("high-quality"));
  assertEqual(calls[0], "http://localhost:9090/v1/reflect/experiences?stats=true&tag=quality%3A9");
  assertEqual(calls[1], "http://localhost:9090/v1/reflect/strategies?tag=quality%3A9&limit=3");
});

test("ReflectClient exposes reflect-named client errors", async () => {
  const client = createReflectClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "NOT_FOUND", message: "experience store not initialized" } }, { status: 404 }) });

  try {
    await client.strategies();
    throw new Error("expected strategies to reject");
  } catch (error) {
    assert(error instanceof ReflectClientError);
    assertEqual(error.status, 404);
    assertEqual(error.message, "experience store not initialized");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
