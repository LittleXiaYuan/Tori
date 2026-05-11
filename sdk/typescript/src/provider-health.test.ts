import { createProviderHealthClient, ProviderHealthClientError } from "./provider-health";

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

test("ProviderHealthClient lists providers with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderHealthClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ providers: [{ id: "moonshot", model: "moonshot-v1-8k" }], mode: "hybrid" });
    },
  });

  const result = await client.list();

  assertEqual(result.mode, "hybrid");
  assertEqual(result.providers[0]?.id, "moonshot");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/providers");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ProviderHealthClient tests providers and reads mode and exec provider with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createProviderHealthClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/test")) return jsonResponse({ success: true });
      if (String(url).endsWith("/mode")) return jsonResponse({ ok: true, mode: "local", bound: false });
      return jsonResponse({ ok: true, exec_provider: "local-qwen", available_providers: ["local-qwen"] });
    },
  });

  const tested = await client.test("local-qwen");
  const mode = await client.mode();
  const exec = await client.exec();

  assertEqual(tested.success, true);
  assertEqual(mode.mode, "local");
  assertEqual(exec.exec_provider, "local-qwen");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/providers/test");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "local-qwen" }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("ProviderHealthClient exposes provider-health nested gateway errors", async () => {
  const client = createProviderHealthClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested provider health failure" } }, { status: 400 }),
  });

  try {
    await client.test("");
    throw new Error("expected test to reject");
  } catch (error) {
    assert(error instanceof ProviderHealthClientError);
    assertEqual(error.name, "ProvidersClientError");
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested provider health failure" } });
    assertEqual(error.message, "nested provider health failure");
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
