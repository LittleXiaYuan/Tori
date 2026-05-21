import { createCostClient, CostClientError } from "./cost";

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

test("CostClient reads summary with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ summary: { calls: 2 }, today_cost: 0.12, month_cost: 1.5 });
    },
  });

  const result = await client.summary();

  assertEqual(result.today_cost, 0.12);
  assertEqual(result.month_cost, 1.5);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/summary");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("CostClient sets budget with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ ok: true });
    },
  });

  const result = await client.setBudget({ daily_limit_usd: 1, monthly_limit_usd: 20 });

  assertEqual(result.ok, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/budget");
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ daily_limit_usd: 1, monthly_limit_usd: 20 }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("CostClient reads task cost timeline breakdown history and alerts", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("/task/timeline")) return jsonResponse([{ cost_usd: 0.01 }]);
      if (String(url).includes("/task?")) return jsonResponse({ task_id: "task-1", total_cost: 0.02 });
      if (String(url).includes("/breakdown")) return jsonResponse({ by_provider: { openai: 0.1 }, by_tier: { fast: 0.1 } });
      if (String(url).includes("/history")) return jsonResponse({ records: [], page: 2, limit: 25 });
      return jsonResponse({ alerts: [{ type: "daily" }], today_cost: 0.1 });
    },
  });

  const task = await client.task("task-1") as { total_cost?: number };
  const timeline = await client.taskTimeline("task-1") as Array<{ cost_usd: number }>;
  const breakdown = await client.breakdown();
  const history = await client.history({ page: 2, limit: 25, model: "gpt-test", provider_id: "p1" }) as { page?: number };
  const alerts = await client.alerts();

  assertEqual(task.total_cost, 0.02);
  assertEqual(timeline[0]?.cost_usd, 0.01);
  assertEqual(breakdown.by_provider?.openai, 0.1);
  assertEqual(history.page, 2);
  assertEqual(alerts.alerts?.length, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/cost/task?id=task-1");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/cost/task/timeline?id=task-1");
  assertEqual(calls[3]?.url, "http://localhost:9090/v1/cost/history?page=2&limit=25&model=gpt-test&provider_id=p1");
});

test("CostClient reads usage and sets quota", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createCostClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (init?.method === "POST") return jsonResponse({ status: "ok" });
      return jsonResponse({ tenant_id: "tenant-1", chat_calls: 2, tokens_used: 123 });
    },
  });

  const usage = await client.usage();
  const quota = await client.setQuota({ tenant_id: "tenant-1", quota: { max_chat_calls: 10, max_tokens_per_day: 1000 } });

  assertEqual(usage.tenant_id, "tenant-1");
  assertEqual(usage.tokens_used, 123);
  assertEqual(quota.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/usage");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/quota");
  assertEqual(calls[1]?.init?.body, JSON.stringify({ tenant_id: "tenant-1", quota: { max_chat_calls: 10, max_tokens_per_day: 1000 } }));
});

test("CostClient throws CostClientError with parsed and text bodies", async () => {
  const jsonClient = createCostClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "id is required" }, { status: 400 }),
  });

  try {
    await jsonClient.task("");
    throw new Error("expected task to reject");
  } catch (error) {
    assert(error instanceof CostClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "id is required" });
    assertEqual(error.message, "id is required");
  }


  const nestedClient = createCostClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "cost task id is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.task("");
    throw new Error("expected task to reject");
  } catch (error) {
    assert(error instanceof CostClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "cost task id is required");
  }

  const textClient = createCostClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("POST required", { status: 405 }),
  });

  try {
    await textClient.setBudget({ daily_limit_usd: 1 });
    throw new Error("expected setBudget to reject");
  } catch (error) {
    assert(error instanceof CostClientError);
    assertEqual(error.status, 405);
    assertEqual(error.body, "POST required");
    assertEqual(error.message, "POST required");
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
