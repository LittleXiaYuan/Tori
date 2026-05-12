import { createReflectExperiencesClient, ReflectExperiencesClientError } from "./reflect-experiences";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ReflectExperiencesClient lists filtered experiences with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createReflectExperiencesClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ experiences: [{ id: "e1", source: "task", category: "sdk", outcome: "success" }], total: 1 }); } });
  const result = await client.list({ q: "sdk", source: "task", category: "sdk", outcome: "success", tag: "quality:9", limit: 5 });
  assertEqual(result.total, 1); assertEqual(result.experiences[0]?.outcome, "success"); assertEqual(calls[0]?.url, "http://localhost:9090/v1/reflect/experiences?q=sdk&source=task&category=sdk&outcome=success&tag=quality%3A9&limit=5"); assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ReflectExperiencesClient reads scoped stats with bearer token", async () => {
  const calls: string[] = [];
  const client = createReflectExperiencesClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url) => { calls.push(String(url)); return jsonResponse({ total: 10, by_outcome: { success: 8 }, recent_7d: 3 }); } });
  const stats = await client.stats({ source: "task", outcome: "success", tag: "quality:9" });
  assertEqual(stats.total, 10); assertEqual(stats.by_outcome?.success, 8); assertEqual(calls[0], "http://localhost:9090/v1/reflect/experiences?stats=true&source=task&outcome=success&tag=quality%3A9");
});

test("ReflectExperiencesClient exposes nested experience errors", async () => {
  const client = createReflectExperiencesClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { code: "NOT_FOUND", message: "experience store not initialized" } }, { status: 404 }) });
  try { await client.list(); throw new Error("expected list to reject"); } catch (error) { assert(error instanceof ReflectExperiencesClientError); assertEqual(error.name, "MissionsClientError"); assertEqual(error.status, 404); assertEqual(error.message, "experience store not initialized"); }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
