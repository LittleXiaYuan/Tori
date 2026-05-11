import { createHeartbeatClient, HeartbeatClientError } from "./heartbeat";

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

test("HeartbeatClient reads status with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createHeartbeatClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ running: true });
    },
  });

  const result = await client.status();

  assertEqual(result.running, true);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/heartbeat");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("HeartbeatClient updates enabled state and interval with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createHeartbeatClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ status: "ok" });
    },
  });

  const result = await client.update({ enabled: true, interval_minutes: 30 });

  assertEqual(result.status, "ok");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/heartbeat");
  assertEqual(calls[0]?.init?.method, "PUT");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ enabled: true, interval_minutes: 30 }));
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("HeartbeatClient triggers heartbeat", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createHeartbeatClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ id: "hb1", status: "ok", summary: "checked" });
    },
  });

  const result = await client.trigger();

  assertEqual(result.id, "hb1");
  assertEqual(result.summary, "checked");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/heartbeat/trigger");
  assertEqual(calls[0]?.init?.method, "POST");
});

test("HeartbeatClient reads logs with limit", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createHeartbeatClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse([{ id: "hb1" }, { id: "hb2" }]);
    },
  });

  const result = await client.logs({ limit: 2 });

  assertEqual(result.length, 2);
  assertEqual(result[1]?.id, "hb2");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/heartbeat/logs?limit=2");
});

test("HeartbeatClient throws HeartbeatClientError with parsed and text bodies", async () => {
  const jsonClient = createHeartbeatClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "heartbeat not configured" }, { status: 500 }),
  });

  try {
    await jsonClient.status();
    throw new Error("expected status to reject");
  } catch (error) {
    assert(error instanceof HeartbeatClientError);
    assertEqual(error.status, 500);
    assertDeepEqual(error.body, { error: "heartbeat not configured" });
    assertEqual(error.message, "heartbeat not configured");
  }

  const textClient = createHeartbeatClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("POST only", { status: 405 }),
  });

  try {
    await textClient.trigger();
    throw new Error("expected trigger to reject");
  } catch (error) {
    assert(error instanceof HeartbeatClientError);
    assertEqual(error.status, 405);
    assertEqual(error.body, "POST only");
    assertEqual(error.message, "POST only");
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
