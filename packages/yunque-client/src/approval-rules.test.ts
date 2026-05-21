import { createApprovalRulesClient, ApprovalRulesClientError } from "./approval-rules";

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

test("ApprovalRulesClient lists rules with bearer token", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalRulesClient({
    baseUrl: "http://localhost:9090/",
    token: "token-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      return jsonResponse({ rules: [{ id: "rule-1", action: "shell", decision: "deny_always" }], total: 1 });
    },
  });

  const result = await client.list();

  assertEqual(result.total, 1);
  assertEqual(result.rules[0]?.id, "rule-1");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals/rules");
  assertEqual(new Headers(calls[0]?.init?.headers).get("authorization"), "Bearer token-123");
});

test("ApprovalRulesClient adds and deletes rules with API key", async () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  const client = createApprovalRulesClient({
    baseUrl: "http://localhost:9090",
    apiKey: "key-123",
    fetch: async (url, init) => {
      calls.push({ url: String(url), init });
      if (init?.method === "DELETE") return jsonResponse({ deleted: true, id: "rule-2" });
      return jsonResponse({ status: "ok", id: "rule-2" });
    },
  });

  const added = await client.add({ id: "rule-2", action: "shell", pattern: "npm*", decision: "allow_once" });
  const deleted = await client.delete("rule-2");

  assertEqual(added.id, "rule-2");
  assertEqual(deleted.deleted, true);
  assertEqual(new Headers(calls[0]?.init?.headers).get("x-api-key"), "key-123");
  assertEqual(calls[0]?.url, "http://localhost:9090/v1/approvals/rules");
  assertEqual(calls[0]?.init?.body, JSON.stringify({ id: "rule-2", action: "shell", pattern: "npm*", decision: "allow_once" }));
  assertEqual(calls[1]?.url, "http://localhost:9090/v1/approvals/rules?id=rule-2");
});

test("ApprovalRulesClient exposes approval-rules-named nested gateway errors", async () => {
  const client = createApprovalRulesClient({
    baseUrl: "http://localhost:9090",
    fetch: async () => jsonResponse({ error: { code: "BAD_REQUEST", message: "nested rule failure" } }, { status: 400 }),
  });

  try {
    await client.add({ action: "shell" });
    throw new Error("expected add to reject");
  } catch (error) {
    assert(error instanceof ApprovalRulesClientError);
    assertEqual(error.name, "ApprovalsClientError");
    assertEqual(error.status, 400);
    assertDeepEqual(error.body, { error: { code: "BAD_REQUEST", message: "nested rule failure" } });
    assertEqual(error.message, "nested rule failure");
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
