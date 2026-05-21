import { createReflectStrategiesClient, ReflectStrategiesClientError } from "./reflect-strategies";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ReflectStrategiesClient reads strategies with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReflectStrategiesClient({ baseUrl: "http://localhost:9090/", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ strategies: "- 推荐: Prefer small slices" }); } });
  const result = await client.strategies({ q: "sdk", source: "task", category: "release", outcome: "success", tag: "quality:9", limit: 3 });
  assert(result.strategies.includes("small slices")); assertEqual(calls[0]?.url, "http://localhost:9090/v1/reflect/strategies?q=sdk&source=task&category=release&outcome=success&tag=quality%3A9&limit=3"); assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ReflectStrategiesClient exposes text strategy errors", async () => {
  const client = createReflectStrategiesClient({ baseUrl: "http://localhost:9090", fetch: async () => new Response("experience store not initialized", { status: 404 }) });
  try { await client.strategies(); throw new Error("expected strategies to reject"); } catch (error) { assert(error instanceof ReflectStrategiesClientError); assertEqual(error.name, "MissionsClientError"); assertEqual(error.status, 404); assertEqual(error.body, "experience store not initialized"); assertEqual(error.message, "experience store not initialized"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
