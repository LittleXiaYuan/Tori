import { createProviderRegistryClient, ProviderRegistryClientError } from "./provider-registry";

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

test("ProviderRegistryClient reads presets and registers providers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderRegistryClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/presets")) return jsonResponse({ presets: [{ id: "deepseek", name: "DeepSeek" }] });
      return jsonResponse({ ok: true, provider_id: "deepseek-deepseek-chat" });
    },
  });

  const presets = await client.presets();
  const registered = await client.register({ preset_id: "deepseek", api_key: "sk-test", model: "deepseek-chat" });

  assertEqual(presets.presets[0]?.id, "deepseek");
  assertEqual(registered.provider_id, "deepseek-deepseek-chat");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/presets");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/providers/register");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ preset_id: "deepseek", api_key: "sk-test", model: "deepseek-chat" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ProviderRegistryClient discovers local and Tori providers with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderRegistryClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/local/discover")) return jsonResponse({ available: true, models: ["qwen3.5:4b"] });
      if (String(url).endsWith("/local/register")) return jsonResponse({ ok: true, provider_id: "local-qwen" });
      return jsonResponse({ ok: true, models: [{ id: "kimi" }], registered: 1 });
    },
  });

  const discovered = await client.discoverLocal({ base_url: "http://127.0.0.1:11434" });
  const registered = await client.registerLocal({ base_url: "http://127.0.0.1:11434", model: "qwen3.5:4b", backend: "ollama" });
  const tori = await client.discoverTori({ autoRegister: true });

  assertEqual(discovered.available, true);
  assertEqual(registered.provider_id, "local-qwen");
  assertEqual(tori.registered, 1);
  assertEqual(calls[0]?.init?.body, JSON.stringify({ base_url: "http://127.0.0.1:11434" }));
  assertEqual(calls[2]?.url, "http://localhost:9090/api/providers/tori/discover?auto_register=true");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ProviderRegistryClient deletes providers", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderRegistryClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ ok: true });
    },
  });

  const result = await client.delete("tori-kimi");

  assertEqual(result.ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/delete");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "tori-kimi" }));
});

test("ProviderRegistryClient exposes provider-registry nested gateway errors", async () => {
  const client = createProviderRegistryClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested provider registry failure" } }, { status: 400 }),
  });

  try {
    await client.register({ preset_id: "deepseek" });
    throw new Error("expected register to reject");
  } catch (error) {
    assert(error instanceof ProviderRegistryClientError);
    assertEqual(error.name, "ProvidersClientError");
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested provider registry failure" } });
    assertEqual(error.message, "nested provider registry failure");
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
