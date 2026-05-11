import { createSetupClient, SetupClientError } from "./setup";

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

function sseResponse(chunks: string[]): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      const encoder = new TextEncoder();
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
      controller.close();
    },
  });
  return new Response(stream, {
    status: 200,
    headers: { "Content-Type": "text/event-stream" },
  });
}

test("SetupClient detects environment and reads health with auth", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/detect")) {
        return jsonResponse({ has_docker: true, has_gpu: false, has_ollama: true });
      }
      return jsonResponse({ providers: [{ id: "ollama", available: true }], has_docker: true });
    },
  });

  const detected = await client.detect();
  const health = await client.health();

  assertEqual(detected.has_docker, true);
  assertEqual(health.providers?.[0]?.id, "ollama");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/detect");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/setup/health");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("SetupClient lists templates and tests provider connectivity", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/templates")) {
        return jsonResponse({ templates: [{ id: "local-first", name: "Local First" }], count: 1 });
      }
      return jsonResponse({ ok: true, provider: { base_url: "http://127.0.0.1:11434", available: true } });
    },
  });

  const templates = await client.templates();
  const tested = await client.testProvider({ base_url: "http://127.0.0.1:11434", model: "qwen3.5:4b" });

  assertEqual(templates.count, 1);
  assertEqual(tested.ok, true);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ base_url: "http://127.0.0.1:11434", model: "qwen3.5:4b" }));
});

test("SetupClient applies templates with overrides", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({
        ok: true,
        status: "applied",
        applied: "hybrid",
        persisted: true,
        restart_required: true,
        env_content: "LLM_MODEL=deepseek-chat",
      });
    },
  });

  const result = await client.apply({
    template_id: "hybrid",
    base_url: "https://api.deepseek.com/v1",
    api_key: "sk-test",
    model: "deepseek-chat",
    overrides: { sandbox_tier: "local" },
  });

  assertEqual(result.restart_required, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/setup/apply");
  assertEqual(
    calls[0]?.init?.body,
    JSON.stringify({
      template_id: "hybrid",
      base_url: "https://api.deepseek.com/v1",
      api_key: "sk-test",
      model: "deepseek-chat",
      overrides: { sandbox_tier: "local" },
    }),
  );
});

test("SetupClient installs optional component with JSON and SSE modes", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createSetupClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (new Headers(init?.headers).get("accept") === "text/event-stream") {
        return sseResponse([
          'data: {"stage":"download","progress":50}\n\n',
          'data: {"stage":"done","progress":100}\n\n',
        ]);
      }
      return jsonResponse({ success: true, message: "installed" });
    },
  });

  const installed = await client.installComponent("python_office");
  const events = [];
  for await (const event of client.installComponentStream("python_office")) events.push(event);

  assertEqual(installed.success, true);
  assertEqual(events.length, 2);
  assertDeepEqual(events[0], { stage: "download", progress: 50 });
  assertEqual(calls[0]?.init?.body, JSON.stringify({ component_id: "python_office" }));
  assertEqual(new Headers(calls[1]?.init?.headers).get("accept"), "text/event-stream");
});

test("SetupClient throws SetupClientError with parsed body", async () => {
  const client = createSetupClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "base_url is required" }, { status: 400 }),
  });

  try {
    await client.testProvider({ base_url: "" });
    throw new Error("expected testProvider to reject");
  } catch (error) {
    assert(error instanceof SetupClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "base_url is required" });
    assertEqual(error.message, "base_url is required");
  }

  const nestedClient = createSetupClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "template_id is required" } }, { status: 400 }),
  });
  try {
    await nestedClient.apply({ template_id: "" });
    throw new Error("expected nested apply to reject");
  } catch (error) {
    assert(error instanceof SetupClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "template_id is required");
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
