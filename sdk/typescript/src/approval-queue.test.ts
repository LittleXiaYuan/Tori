import { createApprovalQueueClient, ApprovalQueueClientError } from "./approval-queue";

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

test("ApprovalQueueClient lists pending approvals with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalQueueClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ approvals: [{ id: "apv-1", status: "pending" }], total: 1 });
    },
  });

  const result = await client.pending();

  assertEqual(result.total, 1);
  assertEqual(result.approvals[0]?.id, "apv-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals?status=pending");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ApprovalQueueClient reads history and filtered lists", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalQueueClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ approvals: [{ id: "apv-2", status: "approved" }], total: 1 });
    },
  });

  const listed = await client.list({ status: "approved" });
  const history = await client.history();

  assertEqual(listed.approvals[0]?.status, "approved");
  assertEqual(history.total, 1);
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals?status=approved");
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/approvals?history=true");
});

test("ApprovalQueueClient approves denies and persists decisions with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalQueueClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/approve")) return jsonResponse({ status: "approved", id: "apv-3" });
      if (String(url).endsWith("/deny")) return jsonResponse({ status: "denied", id: "apv-4" });
      return jsonResponse({ status: "ok", decision: "allow_always" });
    },
  });

  const approved = await client.approve("apv-3");
  const denied = await client.deny("apv-4", "人工拒绝");
  const decided = await client.decide("apv-5", "allow_always");

  assertEqual(approved.status, "approved");
  assertEqual(denied.status, "denied");
  assertEqual(decided.decision, "allow_always");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "apv-3" }));
  assertEqual(calls[1]?.init?.body, JSON.stringify({ id: "apv-4", reason: "人工拒绝" }));
  assertEqual(calls[2]?.init?.body, JSON.stringify({ id: "apv-5", decision: "allow_always" }));
});

test("ApprovalQueueClient exposes approval-queue nested gateway errors", async () => {
  const client = createApprovalQueueClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested queue failure" } }, { status: 400 }),
  });

  try {
    await client.approve("");
    throw new Error("expected approve to reject");
  } catch (error) {
    assert(error instanceof ApprovalQueueClientError);
    assertEqual(error.name, "ApprovalsClientError");
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested queue failure" } });
    assertEqual(error.message, "nested queue failure");
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
