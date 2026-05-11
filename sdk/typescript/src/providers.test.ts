import { createProvidersClient, ProvidersClientError } from "./providers";

declare const process: { exitCode?: number };

function assert(condition: unknown, message?: string): asserts condition {
  if (!condition) throw new Error(message || "assertion failed");
}

function assertEqual(actual: unknown, expected: unknown, message?: string): void {
  if (actual !== expected) throw new Error(message || `expected ${JSON.stringify(actual)} to equal ${JSON.stringify(expected)}`);
}

function assertDeepEqual(actual: unknown, expected: unknown, message?: string): void {
  const actualJson = JSON.stringify(actual);
  const expectedJson = JSON.stringify(expected);
  if (actualJson !== expectedJson) throw new Error(message || `expected ${actualJson} to deep equal ${expectedJson}`);
}

const tests: Array<{ name: string; fn: () => Promise<void> | void }> = [];

function test(name: string, fn: () => Promise<void> | void): void {
  tests.push({ name, fn });
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

test("ProvidersClient lists providers and models with auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProvidersClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/v1/models")) {
        return jsonResponse({ models: [{ id: "kimi", model_id: "moonshot-v1-8k" }] });
      }
      return jsonResponse({ providers: [{ id: "moonshot", model: "moonshot-v1-8k" }], mode: "hybrid" });
    },
  });

  const models = await client.listModels();
  const providers = await client.listProviders();

  assertEqual(models.models[0]?.id, "kimi");
  assertEqual(providers.mode, "hybrid");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/models");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/providers");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ProvidersClient registers provider and tests connectivity", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProvidersClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/register")) {
        return jsonResponse({ ok: true, provider_id: "deepseek-deepseek-chat" });
      }
      return jsonResponse({ success: true });
    },
  });

  const registered = await client.registerProvider({
    preset_id: "deepseek",
    api_key: "sk-test",
    model: "deepseek-chat",
  });
  const tested = await client.testProvider("deepseek-deepseek-chat");

  assertEqual(registered.provider_id, "deepseek-deepseek-chat");
  assertEqual(tested.success, true);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ preset_id: "deepseek", api_key: "sk-test", model: "deepseek-chat" }));
  assertEqual(calls[1]?.init?.body, JSON.stringify({ id: "deepseek-deepseek-chat" }));
});

test("ProvidersClient controls provider lifecycle and session override", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProvidersClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ ok: true, model: "qwen3.5:4b", action: "set" });
    },
  });

  await client.enableProvider("ollama");
  await client.disableProvider("ollama");
  await client.switchModel("ollama", "qwen3.5:4b");
  await client.setSessionProvider({ session_id: "s1", provider_id: "ollama" });
  await client.clearSessionProvider("s1");

  assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/enable");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/providers/disable");
  assertEqual(calls[2]?.init?.body, JSON.stringify({ id: "ollama", model: "qwen3.5:4b" }));
  assertEqual(calls[3]?.init?.body, JSON.stringify({ session_id: "s1", provider_id: "ollama" }));
  assertEqual(calls[4]?.init?.body, JSON.stringify({ session_id: "s1", provider_id: "" }));
});

test("ProvidersClient supports mode, presets, local discovery and exec provider", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProvidersClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/mode")) return jsonResponse({ ok: true, mode: "local", bound: false });
      if (String(url).endsWith("/presets")) return jsonResponse({ presets: [{ id: "deepseek", name: "DeepSeek" }] });
      if (String(url).endsWith("/local/discover")) return jsonResponse({ available: true, models: ["qwen3.5:4b"] });
      if (String(url).endsWith("/local/register")) return jsonResponse({ ok: true, provider_id: "local-qwen3.5:4b" });
      return jsonResponse({ ok: true, exec_provider: "local-qwen3.5:4b", available_providers: ["local-qwen3.5:4b"] });
    },
  });

  const mode = await client.setMode("local");
  const presets = await client.presets();
  const discovered = await client.discoverLocal({ base_url: "http://127.0.0.1:11434" });
  const local = await client.registerLocal({ base_url: "http://127.0.0.1:11434", model: "qwen3.5:4b", backend: "ollama" });
  const exec = await client.setExecProvider("local-qwen3.5:4b");

  assertEqual(mode.mode, "local");
  assertEqual(presets.presets[0]?.id, "deepseek");
  assertEqual(discovered.available, true);
  assertEqual(local.provider_id, "local-qwen3.5:4b");
  assertEqual(exec.exec_provider, "local-qwen3.5:4b");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ mode: "local" }));
  assertEqual(calls[2]?.init?.body, JSON.stringify({ base_url: "http://127.0.0.1:11434" }));
});

test("ProvidersClient supports model CRUD, Tori discovery, provider delete and breaker reset", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProvidersClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("/tori/discover")) return jsonResponse({ ok: true, models: [{ id: "kimi" }], registered: 1 });
      if (String(url).includes("/breaker/reset")) return jsonResponse({ ok: true, reset_count: 2 });
      if (String(url).endsWith("/delete")) return jsonResponse({ ok: true });
      if (init?.method === "DELETE") return jsonResponse({ status: "ok" });
      return jsonResponse({ id: "custom", model_id: "custom-model" });
    },
  });

  const model = await client.addModel({ id: "custom", model_id: "custom-model" });
  const deletedModel = await client.deleteModel("custom");
  const tori = await client.discoverTori({ autoRegister: true });
  const deletedProvider = await client.deleteProvider("tori-kimi");
  const reset = await client.resetBreakers();

  assertEqual(model.id, "custom");
  assertEqual(deletedModel.status, "ok");
  assertEqual(tori.registered, 1);
  assertEqual(deletedProvider.ok, true);
  assertEqual(reset.reset_count, 2);
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/models?id=custom");
  assertEqual(calls[2]?.url, "http://localhost:9090/api/providers/tori/discover?auto_register=true");
  assertEqual(calls[3]?.init?.body, JSON.stringify({ id: "tori-kimi" }));
});

test("ProvidersClient throws ProvidersClientError with parsed body", async () => {
  const client = createProvidersClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "provider registry not available" }, { status: 404 }),
  });

  try {
    await client.listProviders();
    throw new Error("expected listProviders to reject");
  } catch (error) {
    assert(error instanceof ProvidersClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "provider registry not available" });
    assertEqual(error.message, "provider registry not available");
  }
});

let failures = 0;
for (const { name, fn } of tests) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failures > 0) {
  process.exitCode = 1;
} else {
  console.log(`1..${tests.length}`);
}
