import { createIterateClient, IterateClientError } from "./iterate";

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

test("IterateClient lists proposals with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIterateClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ proposals: [{ id: "p1", status: "pending", title: "Add memory" }], count: 1 });
    },
  });

  const result = await client.proposals();

  assertEqual(result.count, 1);
  assertEqual(result.proposals[0]?.id, "p1");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/iterate/proposals");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("IterateClient lists pending proposals with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIterateClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ proposals: [{ id: "p2", status: "pending" }], count: 1 });
    },
  });

  const result = await client.pendingProposals();

  assertEqual(result.proposals[0]?.id, "p2");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/iterate/proposals?status=pending");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("IterateClient approves and rejects proposals", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIterateClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/approve")) return jsonResponse({ status: "approved", id: "p1" });
      return jsonResponse({ status: "rejected", id: "p2" });
    },
  });

  const approved = await client.approve({ id: "p1" });
  const rejected = await client.reject({ id: "p2" });

  assertEqual(approved.status, "approved");
  assertEqual(rejected.status, "rejected");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/iterate/approve");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "p1" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/api/iterate/reject");
});

test("IterateClient triggers cycle and reads status", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createIterateClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/trigger")) return jsonResponse({ status: "ok", cycle: { id: "cycle-1", stopped_by: "complete" } });
      return jsonResponse({ enabled: true, pending_proposals: 2 });
    },
  });

  const trigger = await client.trigger();
  const status = await client.status();

  assertEqual(trigger.status, "ok");
  assertEqual(trigger.cycle?.id, "cycle-1");
  assertEqual(status.enabled, true);
  assertEqual(status.pending_proposals, 2);
  assertEqual(calls[0]?.init?.method, "POST");
  assertEqual(calls[1]?.url, "http://localhost:9090/api/iterate/status");
});

test("IterateClient throws IterateClientError with parsed and text bodies", async () => {
  const jsonClient = createIterateClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "id is required" }, { status: 400 }),
  });

  try {
    await jsonClient.approve({ id: "" });
    throw new Error("expected approve to reject");
  } catch (error) {
    assert(error instanceof IterateClientError);
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: "id is required" });
    assertEqual(error.message, "id is required");
  }

  const textClient = createIterateClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("POST required", { status: 405 }),
  });

  try {
    await textClient.trigger();
    throw new Error("expected trigger to reject");
  } catch (error) {
    assert(error instanceof IterateClientError);
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
