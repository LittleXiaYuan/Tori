import { createProviderControlClient, ProviderControlClientError } from "./provider-control";

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

test("ProviderControlClient enables disables and switches provider models with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderControlClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ ok: true, action: "updated" });
    },
  });

  await client.enable("ollama");
  await client.disable("ollama");
  await client.switchModel("ollama", "qwen3.5:4b");

  assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/enable");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/providers/disable");
  assertEqual(calls[2]?.url, "http://localhost:9090/api/providers/switch-model");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "ollama" }));
  assertEqual(calls[2]?.init?.body, JSON.stringify({ id: "ollama", model: "qwen3.5:4b" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ProviderControlClient manages session overrides mode and exec provider with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderControlClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/mode")) return jsonResponse({ ok: true, mode: "local", bound: false });
      if (String(url).endsWith("/exec")) return jsonResponse({ ok: true, exec_provider: "local-qwen" });
      return jsonResponse({ ok: true });
    },
  });

  await client.setSessionProvider({ session_id: "s1", provider_id: "ollama" });
  await client.clearSessionProvider("s1");
  const mode = await client.setMode("local");
  const exec = await client.setExecProvider("local-qwen");

  assertEqual(mode.mode, "local");
  assertEqual(exec.exec_provider, "local-qwen");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/session");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ session_id: "s1", provider_id: "ollama" }));
  assertEqual(calls[1]?.init?.body, JSON.stringify({ session_id: "s1", provider_id: "" }));
  assertEqual(calls[2]?.init?.body, JSON.stringify({ mode: "local" }));
  assertEqual(calls[3]?.init?.body, JSON.stringify({ provider_id: "local-qwen" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ProviderControlClient exposes provider-control nested gateway errors", async () => {
  const client = createProviderControlClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested provider control failure" } }, { status: 400 }),
  });

  try {
    await client.enable("");
    throw new Error("expected enable to reject");
  } catch (error) {
    assert(error instanceof ProviderControlClientError);
    assertEqual(error.name, "ProvidersClientError");
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested provider control failure" } });
    assertEqual(error.message, "nested provider control failure");
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
