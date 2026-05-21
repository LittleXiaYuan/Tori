import { createLoRAClient, LoRAClientError } from "./lora";

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

test("LoRAClient reads status with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ scheduler: { status: "idle" }, active_model: "base", rolling_success_rate: 0.8 });
    },
  });

  const result = await client.status();

  assertEqual(result.active_model, "base");
  assertEqual(result.rolling_success_rate, 0.8);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/status");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("LoRAClient reads history summary preview and evolution", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("/history")) return jsonResponse({ records: [{ adapter: "a1" }], count: 1 });
      if (String(url).includes("/summary")) return jsonResponse({ summary: { best_score: 0.9 } });
      if (String(url).includes("/preview")) return jsonResponse({ preview: { ready: true, tenant_id: "tenant-1", sample_count: 8 } });
      return jsonResponse({ state: { rolling_success_rate: 0.75 } });
    },
  });

  const history = await client.history();
  const summary = await client.summary() as { summary: { best_score: number } };
  const preview = await client.preview({ tenant_id: "tenant-1" });
  const evolution = await client.evolution() as { state: { rolling_success_rate: number } };

  assertEqual(history.count, 1);
  assertEqual(summary.summary.best_score, 0.9);
  assertEqual(preview.preview.ready, true);
  assertEqual(evolution.state.rolling_success_rate, 0.75);
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/lora/preview?tenant_id=tenant-1");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("LoRAClient triggers training and rollback", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/trigger")) return jsonResponse({ status: "ok", tenant_id: "tenant-2" });
      return jsonResponse({ status: "ok" });
    },
  });

  const triggered = await client.trigger({ tenant_id: "tenant-2" });
  const rolledBack = await client.rollback();

  assertEqual(triggered.status, "ok");
  assertEqual(triggered.tenant_id, "tenant-2");
  assertEqual(rolledBack.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/lora/trigger");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ tenant_id: "tenant-2" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/lora/rollback");
});

test("LoRAClient reads and updates config", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createLoRAClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (init?.method === "GET") return jsonResponse({ config: { min_samples: 10, base_model: "base" } });
      return jsonResponse({ config: { min_samples: 12, base_model: "base" }, status: "updated" });
    },
  });

  const current = await client.config() as { config: { min_samples: number } };
  const updated = await client.updateConfig({ min_samples: 12, min_interval: "1h" }, "PATCH") as { config: { min_samples: number }; status?: string };

  assertEqual(current.config.min_samples, 10);
  assertEqual(updated.status, "updated");
  assertEqual(updated.config.min_samples, 12);
  assertEqual(calls[1]?.init?.method, "PATCH");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ min_samples: 12, min_interval: "1h" }));
});

test("LoRAClient throws LoRAClientError with parsed and text bodies", async () => {
  const jsonClient = createLoRAClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "LoRA scheduler not configured" }, { status: 500 }),
  });

  try {
    await jsonClient.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof LoRAClientError);
    assertEqual(error.status, 500);
    assertDeepEqual(error.body, { error: "LoRA scheduler not configured" });
    assertEqual(error.message, "LoRA scheduler not configured");
  }


  const nestedClient = createLoRAClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "lora training config is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.trigger({});
    throw new Error("expected trigger to reject");
  } catch (error) {
    assert(error instanceof LoRAClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "lora training config is required");
  }

  const textClient = createLoRAClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("method not allowed", { status: 405 }),
  });

  try {
    await textClient.rollback();
    throw new Error("expected rollback to reject");
  } catch (error) {
    assert(error instanceof LoRAClientError);
    assertEqual(error.status, 405);
    assertEqual(error.body, "method not allowed");
    assertEqual(error.message, "method not allowed");
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
