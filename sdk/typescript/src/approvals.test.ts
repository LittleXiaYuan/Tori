import { createApprovalsClient, ApprovalsClientError } from "./approvals";

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

test("ApprovalsClient lists pending approvals with auth and filters", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalsClient({
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

test("ApprovalsClient supports approval and denial actions", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalsClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/approve")) return jsonResponse({ status: "approved", id: "apv-2" });
      return jsonResponse({ status: "denied", id: "apv-3" });
    },
  });

  const approved = await client.approve("apv-2");
  const denied = await client.deny("apv-3", "需要人工复核");

  assertEqual(approved.status, "approved");
  assertEqual(denied.status, "denied");
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "apv-2" }));
  assertEqual(calls[1]?.init?.body, JSON.stringify({ id: "apv-3", reason: "需要人工复核" }));
});

test("ApprovalsClient supports persistent decisions and history", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalsClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (String(url).includes("/decide")) return jsonResponse({ status: "ok", decision: "allow_always" });
      return jsonResponse({ approvals: [{ id: "apv-4", status: "approved" }], total: 1 });
    },
  });

  const decision = await client.decide("apv-4", "allow_always");
  const history = await client.history();

  assertEqual(decision.decision, "allow_always");
  assertEqual(history.approvals[0]?.status, "approved");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "apv-4", decision: "allow_always" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/approvals?history=true");
});

test("ApprovalsClient manages approval rules", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalsClient({
    baseUrl: "http://localhost:9090",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (init?.method === "GET") return jsonResponse({ rules: [{ id: "rule-1", action: "shell" }], total: 1 });
      if (init?.method === "DELETE") return jsonResponse({ deleted: true });
      return jsonResponse({ status: "ok", id: "rule-2" });
    },
  });

  const rules = await client.rules();
  const added = await client.addRule({ id: "rule-2", action: "shell", decision: "deny_always" });
  const deleted = await client.deleteRule("rule-2");

  assertEqual(rules.total, 1);
  assertEqual(added.id, "rule-2");
  assertEqual(deleted.deleted, true);
  assertEqual(calls[1]?.init?.body, JSON.stringify({ id: "rule-2", action: "shell", decision: "deny_always" }));
  assertEqual(calls[2]?.url, "http://localhost:9090/v1/approvals/rules?id=rule-2");
});

test("ApprovalsClient throws ApprovalsClientError with parsed body", async () => {
  const client = createApprovalsClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: "approval system not available" }, { status: 404 }),
  });

  try {
    await client.list();
    throw new Error("expected list to reject");
  } catch (error) {
    assert(error instanceof ApprovalsClientError);
    assertEqual(error.status, 404);
    assertDeepEqual(error.body, { error: "approval system not available" });
    assertEqual(error.message, "approval system not available");
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
