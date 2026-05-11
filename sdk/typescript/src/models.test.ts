import { createModelsClient, ModelsClientError } from "./models";

declare const process: { exitCode?: number };
function assert(condition: unknown, message?: string): asserts condition { if (!condition) throw new Error(message || "assertion failed"); }
function assertEqual(actual: unknown, expected: unknown, message?: string): void { if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`); }
const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];
function test(name: string, fn: () => Promise<void> | void): void { tests.push({ name, fn }); }
function jsonResponse(body: unknown, init?: ResponseInit): Response { return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" }, ...init }); }

test("ModelsClient lists models through models facade", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createModelsClient({ baseUrl: "http://localhost:9090", token: "token-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); return jsonResponse({ models: [{ id: "kimi", model_id: "moonshot-v1-8k" }] }); } });

  const result = await client.listModels();

  assertEqual(result.models[0]?.model_id, "moonshot-v1-8k");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/models");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ModelsClient adds and deletes models without generated SDK", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createModelsClient({ baseUrl: "http://localhost:9090", apiKey: "key-123", fetch: async (url, init) => { calls.push({ url: String(url), init }); if (init?.method === "DELETE") return jsonResponse({ status: "ok" }); return jsonResponse({ id: "custom", model_id: "custom-model" }); } });

  const added = await client.addModel({ id: "custom", model_id: "custom-model" });
  const deleted = await client.deleteModel("custom");

  assertEqual(added.id, "custom");
  assertEqual(deleted.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/models");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "custom", model_id: "custom-model" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/models?id=custom");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ModelsClient exposes models-named errors", async () => {
  const client = createModelsClient({ baseUrl: "http://localhost:9090", fetch: async () => jsonResponse({ error: { message: "model not found" } }, { status: 404 }) });

  try {
    await client.deleteModel("missing");
    throw new Error("expected deleteModel to reject");
  } catch (error) {
    assert(error instanceof ModelsClientError);
    assertEqual(error.status, 404);
    assertEqual(error.message, "model not found");
  }
});

let failures = 0; for (const { name, fn } of tests) { try { await fn(); console.log(`ok - ${name}`); } catch (error) { failures += 1; console.error(`not ok - ${name}`); console.error(error); } }
if (failures > 0) process.exitCode = 1; else console.log(`1..${tests.length}`);
