import { createAuditClient, AuditClientError } from "./audit";

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

test("AuditClient tails audit chain with bearer token and filters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ records: [{ id: "r1", type: "system", actor: "tenant" }], count: 1 });
    },
  });

  const result = await client.tail({ n: 10, type: "system", actor: "tenant" });

  assertEqual(result.count, 1);
  assertEqual(result.records[0]?.id, "r1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/audit/tail?n=10&type=system&actor=tenant");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("AuditClient verifies chain with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ valid: true, checked: 12, chain_length: 12 });
    },
  });

  const result = await client.verify();

  assertEqual(result.valid, true);
  assertEqual(result.checked, 12);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/audit/verify");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
});

test("AuditClient reads stats", async () => {
  const client = createAuditClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ total: 12, by_type: { system: 10 } }),
  });

  const result = await client.stats() as { total?: number; by_type?: Record<string, number> };

  assertEqual(result.total, 12);
  assertEqual(result.by_type?.system, 10);
});

test("AuditClient reads task audit trail with date and type", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createAuditClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ entries: [{ operation: "nl_config", result: "ok", actor: "default" }], count: 1 });
    },
  });

  const result = await client.trail({ date: "2026-05-11", type: "nl_config" });

  assertEqual(result.count, 1);
  assertEqual(result.entries[0]?.operation, "nl_config");
  assertEqual(calls[0]?.url, "http://localhost:9090/api/audit/trail?date=2026-05-11&type=nl_config");
});

test("AuditClient throws AuditClientError with parsed and text bodies", async () => {
  const jsonClient = createAuditClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "audit not configured" }, { status: 503 }),
  });

  try {
    await jsonClient.tail();
    throw new Error("expected tail to reject");
  } catch (error) {
    assert(error instanceof AuditClientError);
    assertEqual(error.status, 503);
    assertDeepEqual(error.body, { error: "audit not configured" });
    assertEqual(error.message, "audit not configured");
  }


  const nestedClient = createAuditClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "audit task id is required" } }, { status: 400 }),
  });

  try {
    await nestedClient.trail({ date: "" });
    throw new Error("expected trail to reject");
  } catch (error) {
    assert(error instanceof AuditClientError);
    assertEqual(error.status, 400);
    assertEqual(error.message, "audit task id is required");
  }

  const textClient = createAuditClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => new Response("unauthorized", { status: 401 }),
  });

  try {
    await textClient.verify();
    throw new Error("expected verify to reject");
  } catch (error) {
    assert(error instanceof AuditClientError);
    assertEqual(error.status, 401);
    assertEqual(error.body, "unauthorized");
    assertEqual(error.message, "unauthorized");
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
